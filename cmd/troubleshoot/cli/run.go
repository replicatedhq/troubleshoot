package cli

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cursor "github.com/ahmetalpbalkan/go-cursor"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/mattn/go-isatty"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	troubleshootclientsetscheme "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
	"github.com/replicatedhq/troubleshoot/pkg/specs"
	"github.com/spf13/viper"
	spin "github.com/tj/go-spin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	httpClient *http.Client
)

func runTroubleshoot(v *viper.Viper, arg string) error {
	fmt.Print(cursor.Hide())
	defer fmt.Print(cursor.Show())

	if v.GetBool("allow-insecure-connections") || v.GetBool("insecure-skip-tls-verify") {
		httpClient = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	} else {
		httpClient = http.DefaultClient
	}

	collectorContent, err := loadSpec(v, arg)
	if err != nil {
		return errors.Wrap(err, "failed to load collector spec")
	}

	multidocs := strings.Split(string(collectorContent), "\n---\n")

	// we suppory both raw collector kinds and supportbundle kinds here
	supportBundleSpec, err := parseSupportBundleFromDoc([]byte(multidocs[0]))
	if err != nil {
		return errors.Wrap(err, "failed to parse collector")
	}

	troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	additionalRedactors := &troubleshootv1beta2.Redactor{}
	for idx, redactor := range v.GetStringSlice("redactors") {
		redactorContent, err := loadSpec(v, redactor)
		if err != nil {
			return errors.Wrapf(err, "failed to load redactor spec #%d", idx)
		}
		redactorContent, err = docrewrite.ConvertToV1Beta2(redactorContent)
		if err != nil {
			return errors.Wrap(err, "failed to convert to v1beta2")
		}
		obj, _, err := decode([]byte(redactorContent), nil, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to parse redactors %s", redactor)
		}
		loopRedactors, ok := obj.(*troubleshootv1beta2.Redactor)
		if !ok {
			return fmt.Errorf("%s is not a troubleshootv1beta2 redactor type", redactor)
		}
		if loopRedactors != nil {
			additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, loopRedactors.Spec.Redactors...)
		}
	}

	for i, additionalDoc := range multidocs {
		if i == 0 {
			continue
		}
		additionalDoc, err := docrewrite.ConvertToV1Beta2([]byte(additionalDoc))
		if err != nil {
			return errors.Wrap(err, "failed to convert to v1beta2")
		}
		obj, _, err := decode(additionalDoc, nil, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to parse additional doc %d", i)
		}
		multidocRedactors, ok := obj.(*troubleshootv1beta2.Redactor)
		if !ok {
			continue
		}
		additionalRedactors.Spec.Redactors = append(additionalRedactors.Spec.Redactors, multidocRedactors.Spec.Redactors...)
	}

	s := spin.New()
	finishedCh := make(chan bool, 1)
	progressChan := make(chan interface{}, 0) // non-zero buffer can result in missed messages
	isFinishedChClosed := false
	go func() {
		currentDir := ""
		for {
			select {
			case msg := <-progressChan:
				switch msg := msg.(type) {
				case error:
					c := color.New(color.FgHiRed)
					c.Println(fmt.Sprintf("%s\r * %v", cursor.ClearEntireLine(), msg))
				case string:
					currentDir = filepath.Base(msg)
				}
			case <-finishedCh:
				fmt.Printf("\r%s\r", cursor.ClearEntireLine())
				return
			case <-time.After(time.Millisecond * 100):
				if currentDir == "" {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s", cursor.ClearEntireLine(), s.Next())
				} else {
					fmt.Printf("\r%s \033[36mCollecting support bundle\033[m %s %s", cursor.ClearEntireLine(), s.Next(), currentDir)
				}
			}
		}
	}()
	defer func() {
		if !isFinishedChClosed {
			close(finishedCh)
		}
	}()

	archivePath, err := runCollectors(v, supportBundleSpec.Spec.Collectors, additionalRedactors, progressChan)
	if err != nil {
		return errors.Wrap(err, "run collectors")
	}

	fmt.Printf("\r%s\r", cursor.ClearEntireLine())

	// upload if needed
	fileUploaded := false
	if len(supportBundleSpec.Spec.AfterCollection) > 0 {
		for _, ac := range supportBundleSpec.Spec.AfterCollection {
			if ac.UploadResultsTo != nil {
				if err := uploadSupportBundle(ac.UploadResultsTo, archivePath); err != nil {
					c := color.New(color.FgHiRed)
					c.Printf("%s\r * Failed to upload support bundle: %v\n", cursor.ClearEntireLine(), err)
				} else {
					fileUploaded = true
				}
			} else if ac.Callback != nil {
				if err := callbackSupportBundleAPI(ac.Callback, archivePath); err != nil {
					c := color.New(color.FgHiRed)
					c.Printf("%s\r * Failed to notify API that support bundle has been uploaded: %v\n", cursor.ClearEntireLine(), err)
				}
			}
		}

	}

	// perform analysis, if possible
	if len(supportBundleSpec.Spec.Analyzers) > 0 {
		tmpDir, err := ioutil.TempDir("", "troubleshoot")
		if err != nil {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * Failed to make directory for analysis: %v\n", cursor.ClearEntireLine(), err)
		}

		f, err := os.Open(archivePath)
		if err != nil {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * Failed to open support bundle for analysis: %v\n", cursor.ClearEntireLine(), err)

		}
		if err := analyzer.ExtractTroubleshootBundle(f, tmpDir); err != nil {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * Failed to extract support bundle for analysis: %v\n", cursor.ClearEntireLine(), err)
		}

		analyzeResults, err := analyzer.AnalyzeLocal(tmpDir, supportBundleSpec.Spec.Analyzers)
		if err != nil {
			c := color.New(color.FgHiRed)
			c.Printf("%s\r * Failed to analyze support bundle: %v\n", cursor.ClearEntireLine(), err)
		}

		interactive := isatty.IsTerminal(os.Stdout.Fd())

		if interactive {
			close(finishedCh) // this removes the spinner
			isFinishedChClosed = true

			if err := showInteractiveResults(supportBundleSpec.Name, analyzeResults); err != nil {
				interactive = false
			}
		}

		if !interactive {
			data := convert.FromAnalyzerResult(analyzeResults)
			formatted, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				c := color.New(color.FgHiRed)
				c.Printf("%s\r * Failed to format analysis: %v\n", cursor.ClearEntireLine(), err)
			}

			fmt.Printf("%s", formatted)
		}
	}

	if !fileUploaded {
		msg := archivePath
		if appName := supportBundleSpec.Labels["applicationName"]; appName != "" {
			f := `A support bundle for %s has been created in this directory
named %s. Please upload it on the Troubleshoot page of
the %s Admin Console to begin analysis.`
			msg = fmt.Sprintf(f, appName, archivePath, appName)
		}

		fmt.Printf("%s\n", msg)

		return nil
	}

	fmt.Printf("\r%s\r", cursor.ClearEntireLine())
	if fileUploaded {
		fmt.Printf("A support bundle has been created and uploaded to your cluster for analysis. Please visit the Troubleshoot page to continue.\n")
		fmt.Printf("A copy of this support bundle was written to the current directory, named %q\n", archivePath)
	} else {
		fmt.Printf("A support bundle has been created in the current directory named %q\n", archivePath)
	}
	return nil
}

func loadSpec(v *viper.Viper, arg string) ([]byte, error) {
	var err error
	if strings.HasPrefix(arg, "secret/") {
		// format secret/namespace-name/secret-name
		pathParts := strings.Split(arg, "/")
		if len(pathParts) != 3 {
			return nil, errors.Errorf("path %s must have 3 components", arg)
		}

		spec, err := specs.LoadFromSecret(pathParts[1], pathParts[2], "support-bundle-spec")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get spec from secret")
		}

		return spec, nil
	}

	if _, err = os.Stat(arg); err == nil {
		b, err := ioutil.ReadFile(arg)
		if err != nil {
			return nil, errors.Wrap(err, "read spec file")
		}

		return b, nil
	} else if !util.IsURL(arg) {
		return nil, fmt.Errorf("%s is not a URL and was not found (err %s)", arg, err)
	}

	spec, err := loadSpecFromURL(v, arg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get spec from URL")
	}
	return spec, nil
}

func loadSpecFromURL(v *viper.Viper, arg string) ([]byte, error) {
	for {
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			return nil, errors.Wrap(err, "make request")
		}
		req.Header.Set("User-Agent", "Replicated_Troubleshoot/v1beta1")
		req.Header.Set("Bundle-Upload-Host", fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host))
		resp, err := httpClient.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "x509") && httpClient == http.DefaultClient && canTryInsecure(v) {
				httpClient = &http.Client{Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}}
				continue
			}
			return nil, errors.Wrap(err, "execute request")
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "read responce body")
		}

		return body, nil
	}
}

func parseSupportBundleFromDoc(doc []byte) (*troubleshootv1beta2.SupportBundle, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	troubleshootclientsetscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode(doc, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse document")
	}

	collector, ok := obj.(*troubleshootv1beta2.Collector)
	if ok {
		supportBundle := troubleshootv1beta2.SupportBundle{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "troubleshoot.sh/v1beta2",
				Kind:       "SupportBundle",
			},
			ObjectMeta: collector.ObjectMeta,
			Spec: troubleshootv1beta2.SupportBundleSpec{
				Collectors:      collector.Spec.Collectors,
				Analyzers:       []*troubleshootv1beta2.Analyze{},
				AfterCollection: collector.Spec.AfterCollection,
			},
		}

		return &supportBundle, nil
	}

	supportBundle, ok := obj.(*troubleshootv1beta2.SupportBundle)
	if ok {
		return supportBundle, nil
	}

	return nil, errors.New("spec was not parseable as a troubleshoot kind")
}

func canTryInsecure(v *viper.Viper) bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return false
	}
	prompt := promptui.Prompt{
		Label:     "Connection appears to be insecure. Would you like to attempt to create a support bundle anyway?",
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		return false
	}

	return true
}

func runCollectors(v *viper.Viper, collectors []*troubleshootv1beta2.Collect, additionalRedactors *troubleshootv1beta2.Redactor, progressChan chan interface{}) (string, error) {
	bundlePath, err := ioutil.TempDir("", "troubleshoot")
	if err != nil {
		return "", errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(bundlePath)

	if err = writeVersionFile(bundlePath); err != nil {
		return "", errors.Wrap(err, "write version file")
	}

	collectSpecs := make([]*troubleshootv1beta2.Collect, 0, 0)
	collectSpecs = append(collectSpecs, collectors...)
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}})
	collectSpecs = ensureCollectorInList(collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}})

	config, err := k8sutil.GetRESTConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	var cleanedCollectors collect.Collectors
	for _, desiredCollector := range collectSpecs {
		collector := collect.Collector{
			Redact:       true,
			Collect:      desiredCollector,
			ClientConfig: config,
			Namespace:    v.GetString("namespace"),
		}
		cleanedCollectors = append(cleanedCollectors, &collector)
	}

	if err := cleanedCollectors.CheckRBAC(context.Background()); err != nil {
		return "", errors.Wrap(err, "failed to check RBAC for collectors")
	}

	foundForbidden := false
	for _, c := range cleanedCollectors {
		for _, e := range c.RBACErrors {
			foundForbidden = true
			progressChan <- e
		}
	}

	if foundForbidden && !v.GetBool("collect-without-permissions") {
		return "", errors.New("insufficient permissions to run all collectors")
	}

	globalRedactors := []*troubleshootv1beta2.Redact{}
	if additionalRedactors != nil {
		globalRedactors = additionalRedactors.Spec.Redactors
	}

	// Run preflights collectors synchronously
	for _, collector := range cleanedCollectors {
		if len(collector.RBACErrors) > 0 {
			// don't skip clusterResources collector due to RBAC issues
			if collector.Collect.ClusterResources == nil {
				progressChan <- fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.GetDisplayName())
				continue
			}
		}

		progressChan <- collector.GetDisplayName()

		result, err := collector.RunCollectorSync(globalRedactors)
		if err != nil {
			progressChan <- fmt.Errorf("failed to run collector %q: %v", collector.GetDisplayName(), err)
			continue
		}

		if result != nil {
			err = saveCollectorOutput(result, bundlePath, collector)
			if err != nil {
				progressChan <- fmt.Errorf("failed to parse collector spec %q: %v", collector.GetDisplayName(), err)
				continue
			}
		}
	}

	filename, err := findFileName("support-bundle-"+time.Now().Format("2006-01-02T15:04:05"), "tar.gz")
	if err != nil {
		return "", errors.Wrap(err, "find file name")
	}

	if err := tarSupportBundleDir(bundlePath, filename); err != nil {
		return "", errors.Wrap(err, "create bundle file")
	}

	return filename, nil
}

func saveCollectorOutput(output map[string][]byte, bundlePath string, c *collect.Collector) error {
	for filename, maybeContents := range output {
		if c.Collect.Copy != nil {
			err := untarAndSave(maybeContents, filepath.Join(bundlePath, filepath.Dir(filename)))
			if err != nil {
				return errors.Wrap(err, "extract copied files")
			}
			continue
		}
		fileDir, fileName := filepath.Split(filename)
		outPath := filepath.Join(bundlePath, fileDir)

		if err := os.MkdirAll(outPath, 0777); err != nil {
			return errors.Wrap(err, "create output file")
		}

		if err := writeFile(filepath.Join(outPath, fileName), maybeContents); err != nil {
			return errors.Wrap(err, "write collector output")
		}
	}

	return nil
}

func untarAndSave(tarFile []byte, bundlePath string) error {
	keys := make([]string, 0)
	dirs := make(map[string]*tar.Header)
	files := make(map[string][]byte)
	fileHeaders := make(map[string]*tar.Header)
	tarReader := tar.NewReader(bytes.NewBuffer(tarFile))
	//Extract and separate tar contentes in file and folders, keeping header info from each one.
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		switch header.Typeflag {
		case tar.TypeDir:
			dirs[header.Name] = header
		case tar.TypeReg:
			file := new(bytes.Buffer)
			_, err = io.Copy(file, tarReader)
			if err != nil {
				return err
			}
			files[header.Name] = file.Bytes()
			fileHeaders[header.Name] = header
		default:
			return fmt.Errorf("Tar file entry %s contained unsupported file type %v", header.Name, header.FileInfo().Mode())
		}
	}
	//Create directories from base path: <namespace>/<pod name>/containerPath
	if err := os.MkdirAll(filepath.Join(bundlePath), 0777); err != nil {
		return errors.Wrap(err, "create output file")
	}
	//Order folders stored in variable keys to start always by parent folder. That way folder info is preserved.
	for k := range dirs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	//Orderly create folders.
	for _, k := range keys {
		if err := os.Mkdir(filepath.Join(bundlePath, k), dirs[k].FileInfo().Mode().Perm()); err != nil {
			return errors.Wrap(err, "create output file")
		}
	}
	//Populate folders with respective files and its permissions stored in the header.
	for k, v := range files {
		if err := ioutil.WriteFile(filepath.Join(bundlePath, k), v, fileHeaders[k].FileInfo().Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
func uploadSupportBundle(r *troubleshootv1beta2.ResultRequest, archivePath string) error {
	contentType := getExpectedContentType(r.URI)
	if contentType != "" && contentType != "application/tar+gzip" {
		return fmt.Errorf("cannot upload content type %s", contentType)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return errors.Wrap(err, "open file")
	}
	defer f.Close()

	fileStat, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "stat file")
	}

	req, err := http.NewRequest(r.Method, r.URI, f)
	if err != nil {
		return errors.Wrap(err, "create request")
	}
	req.ContentLength = fileStat.Size()
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	// send redaction report
	if r.RedactURI != "" {
		type PutSupportBundleRedactions struct {
			Redactions redact.RedactionList `json:"redactions"`
		}

		redactBytes, err := json.Marshal(PutSupportBundleRedactions{Redactions: redact.GetRedactionList()})
		if err != nil {
			return errors.Wrap(err, "get redaction report")
		}

		req, err := http.NewRequest("PUT", r.RedactURI, bytes.NewReader(redactBytes))
		if err != nil {
			return errors.Wrap(err, "create redaction report request")
		}
		req.ContentLength = int64(len(redactBytes))

		resp, err := httpClient.Do(req)
		if err != nil {
			return errors.Wrap(err, "execute redaction request")
		}

		if resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected redaction status code %d", resp.StatusCode)
		}
	}

	return nil
}

func getExpectedContentType(uploadURL string) string {
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("Content-Type")
}

func callbackSupportBundleAPI(r *troubleshootv1beta2.ResultRequest, archivePath string) error {
	req, err := http.NewRequest(r.Method, r.URI, nil)
	if err != nil {
		return errors.Wrap(err, "create request")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func tarSupportBundleDir(inputDir, outputFilename string) error {
	tarGz := archiver.TarGz{
		Tar: &archiver.Tar{
			ImplicitTopLevelFolder: false,
		},
	}

	paths := []string{
		filepath.Join(inputDir, VersionFilename), // version file should be first in tar archive for quick extraction
	}

	topLevelFiles, err := ioutil.ReadDir(inputDir)
	if err != nil {
		return errors.Wrap(err, "list bundle directory contents")
	}
	for _, f := range topLevelFiles {
		if f.Name() == VersionFilename {
			continue
		}
		paths = append(paths, filepath.Join(inputDir, f.Name()))
	}

	if err := tarGz.Archive(paths, outputFilename); err != nil {
		return errors.Wrap(err, "create archive")
	}

	return nil
}

type CollectorFailure struct {
	Collector *troubleshootv1beta2.Collect
	Failure   string
}
