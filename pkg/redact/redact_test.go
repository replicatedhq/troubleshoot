package redact

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func Test_Redactors(t *testing.T) {
	// Ensure tokenization is disabled for backward compatibility tests
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	ResetGlobalTokenizer()
	defer ResetRedactionList() // Clean up global redaction list
	original := `[
		{
		  "metadata": {
			"name": "awesome-api",
			"namespace": "default",
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"awesome-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"awesome-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"JWT_SIGNING_KEY\",\"value\":\"jwt-signing-key\"},{\"name\":\"TSED_SUPPRESS_ACCESSLOG\",\"value\":\"1\"},{\"name\":\"PINO_LOG_PRETTY\",\"value\":\"1\"},{\"name\":\"PINO_LOG_LEVEL\",\"value\":\"debug\"},{\"name\":\"NODE_ENV\",\"value\":\"development\"},{\"name\":\"MYSQL_HOST\",\"value\":\"mysql\"},{\"name\":\"MYSQL_USER\",\"value\":\"mysqluser\"},{\"name\":\"MYSQL_PASSWORD\",\"value\":\"password\"},{\"name\":\"MYSQL_PORT\",\"value\":\"3306\"},{\"name\":\"MYSQL_DATABASE\",\"value\":\"mysqldb\"},{\"name\":\"CUSTOMER_AVATAR_S3_BUCKET\",\"value\":\"products-dev-avatars-3\"},{\"name\":\"SUPPORT_BUNDLE_S3_BUCKET\",\"value\":\"products-dev-supportbundles\"},{\"name\":\"APP_RELEASE_S3_BUCKET\",\"value\":\"products-dev-releases\"},{\"name\":\"GRAPHQL_VENDOR_ENDPOINT\",\"value\":\"http://awesome-api:8013/graphql\"},{\"name\":\"GRAPHQL_PREM_ENDPOINT\",\"value\":\"http://awesome-api-prem:8033/graphql\"},{\"name\":\"ANALYZE_ENDPOINT\",\"value\":\"http://lazy-api:3000\"},{\"name\":\"GITHUB_CLIENT_ID\",\"value\":\"Iv1.64993a1aeb9575e0\"},{\"name\":\"GITHUB_PRIVATE_KEY_FILENAME\",\"value\":\"/secret-mounts/github-app-private-key--dev-only.pem\"},{\"name\":\"GITHUB_INTEGRATION_ID\",\"value\":\"7888\"},{\"name\":\"INSTALL_URL\",\"value\":\"http://localhost:8090\"},{\"name\":\"AWS_ACCESS_KEY_ID\",\"value\":\"fake\"},{\"name\":\"AWS_SECRET_ACCESS_KEY\",\"value\":\"fake\"},{\"name\":\"AWS_REGION\",\"value\":\"notaregion\"},{\"name\":\"AWS_OWNER_ACCOUNT\",\"value\":\"fake\"},{\"name\":\"S3_ENDPOINT\",\"value\":\"http://s3:4569\"},{\"name\":\"SERVER_MODE\",\"value\":\"vendor\"},{\"name\":\"NEW_RELIC_APP_NAME\",\"value\":\"awesome-api-vendor\"}],\"image\":\"localhost:32000/awesome-api:e9a281f7@sha256:6e988461ffce2bac3561234f736b9a504bfda1911fa6432b90e6bbb16f67f925\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"awesome-api\",\"ports\":[{\"containerPort\":3000,\"name\":\"awesome-api\"}],\"readinessProbe\":{\"failureThreshold\":3,\"httpGet\":{\"path\":\"/healthz\",\"port\":3000,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":2,\"periodSeconds\":2,\"successThreshold\":1,\"timeoutSeconds\":1}}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "awesome-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "awesome-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "awesome-api",
					"image": "localhost:32000/awesome-api:e9a281f7@sha256:6e988461ffce2bac3561234f736b9a504bfda1911fa6432b90e6bbb16f67f925",
					"ports": [
					  {
						"name": "awesome-api",
						"containerPort": 3000,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "DB_CONNECTION_STRING",
						"value": "dbuser:thisisasecret@tcp(dbserver.org:3309)/blog_production"
					  },
					  {
						"name": "POSTGRES_CONNECTION_STRING",
						"value": "Provider=PostgreSQL OLE DB Provider;Data Source=myServerAddress;location=myDataBase;User ID=myUsername;password=myPassword;timeout=1000;"
					  },
					  {
						"name": "POSTGRES_CONNECTION_STRING_2",
						"value": "postgres://pg_user:pg_password@pg_host:5432/pg_database"
					  },
					  {
						"name": "MYSQL_CONNECTION_STRING",
						"value": "Server=myServerAddress;Database=myDataBase;Uid=myUsername;Pwd=myPassword;UseProcedureBodies=False;"
					  },
					  {
						"name": "JWT_SIGNING_KEY",
						"value": "jwt-signing-key"
					  },
					  {
						"name": "TSED_SUPPRESS_ACCESSLOG",
						"value": "1"
					  },
					  {
						"name": "PINO_LOG_PRETTY",
						"value": "1"
					  },
					  {
						"name": "PINO_LOG_LEVEL",
						"value": "debug"
					  },
					  {
						"name": "NODE_ENV",
						"value": "development"
					  },
					  {
						"name": "MYSQL_HOST",
						"value": "mysql"
					  },
					  {
						"name": "MYSQL_USER",
						"value": "mysqluser"
					  },
					  {
						"name": "MYSQL_PASSWORD",
						"value": "password"
					  },
					  {
						"name": "MYSQL_PORT",
						"value": "3306"
					  },
					  {
						"name": "MYSQL_DATABASE",
						"value": "mysqldb"
					  },
					  {
						"name": "CUSTOMER_AVATAR_S3_BUCKET",
						"value": "products-dev-avatars-3"
					  },
					  {
						"name": "SUPPORT_BUNDLE_S3_BUCKET",
						"value": "products-dev-supportbundles"
					  },
					  {
						"name": "APP_RELEASE_S3_BUCKET",
						"value": "products-dev-releases"
					  },
					  {
						"name": "GRAPHQL_VENDOR_ENDPOINT",
						"value": "http://awesome-api:8013/graphql"
					  },
					  {
						"name": "GRAPHQL_PREM_ENDPOINT",
						"value": "http://awesome-api-prem:8033/graphql"
					  },
					  {
						"name": "ANALYZE_ENDPOINT",
						"value": "http://lazy-api:3000"
					  },
					  {
						"name": "GITHUB_CLIENT_ID",
						"value": "Iv1.64993a1aeb9575e0"
					  },
					  {
						"name": "GITHUB_PRIVATE_KEY_FILENAME",
						"value": "/secret-mounts/github-app-private-key--dev-only.pem"
					  },
					  {
						"name": "GITHUB_INTEGRATION_ID",
						"value": "7888"
					  },
					  {
						"name": "INSTALL_URL",
						"value": "http://localhost:8090"
					  },
					  {
						"name": "AWS_ACCESS_KEY_ID",
						"value": "fake"
					  },
					  {
						"name": "AWS_SECRET_ACCESS_KEY",
						"value": "fake"
					  },
					  {
						"name": "AWS_REGION",
						"value": "notaregion"
					  },
					  {
						"name": "AWS_OWNER_ACCOUNT",
						"value": "fake"
					  },
					  {
						"name": "S3_ENDPOINT",
						"value": "http://s3:4569"
					  },
					  {
						"name": "SERVER_MODE",
						"value": "vendor"
					  },
					  {
						"name": "NEW_RELIC_APP_NAME",
						"value": "awesome-api-vendor"
					  }
					],
					"resources": {},
					"readinessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 3000,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 2,
					  "timeoutSeconds": 1,
					  "periodSeconds": 2,
					  "successThreshold": 1,
					  "failureThreshold": 3
					},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:33Z",
				"lastTransitionTime": "2019-07-15T21:15:33Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "gatekeeping-api",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/gatekeeping-api",
			"uid": "aeb84dc6-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197507",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"gatekeeping-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"gatekeeping-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"args\":[\"serve\"],\"env\":[{\"name\":\"GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api-prem:3000/graphql\"}],\"image\":\"somebigbank/titled\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"gatekeeping-api\",\"ports\":[{\"containerPort\":3000,\"name\":\"gatekeeping-api\"}]}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "gatekeeping-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				"skaffold.dev/builder": "local",
				"skaffold.dev/cleanup": "true",
				"skaffold.dev/deployer": "kustomize",
				"skaffold.dev/docker-api-version": "1.39",
				"skaffold.dev/tag-policy": "git-commit",
				"skaffold.dev/tail": "true"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "gatekeeping-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "gatekeeping-api",
					"image": "somebigbank/titled",
					"args": [
					  "serve"
					],
					"ports": [
					  {
						"name": "gatekeeping-api",
						"containerPort": 3000,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api-prem:3000/graphql"
					  }
					],
					"resources": {},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:34Z",
				"lastTransitionTime": "2019-07-15T21:15:34Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "cranky-api",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/cranky-api",
			"uid": "aebb750c-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197557",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"cranky-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"cranky-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"ELASTICSEARCH_NODES\",\"value\":\"http://esuser:thisisnotasecret@elasticsearch:9200\"},{\"name\":\"MAX_LOGIN_ATTEMPTS\",\"value\":\"7\"},{\"name\":\"LICENSE_SIGNING_KEY\",\"value\":\"\"},{\"name\":\"JWT_SIGNING_KEY\",\"value\":\"jwt-signing-key\"},{\"name\":\"GITHUB_CLIENT_ID\",\"value\":\"9a92961100c3bd14b991\"},{\"name\":\"GITHUB_CLIENT_SECRET\",\"value\":\"not-a-secret\"},{\"name\":\"LICENSE_API_ENDPOINT\",\"value\":\"http://boring-api:3000\"},{\"name\":\"vendor_assets_bucket\",\"value\":\"replicaetd-vendor-assets-dev\"},{\"name\":\"vendor_web_host\",\"value\":\"http://localhost:8080\"},{\"name\":\"download_web_host\",\"value\":\"http://localhost:8080\"},{\"name\":\"SPEC_V1_PATH\",\"value\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v1\"},{\"name\":\"SPEC_V2_PATH\",\"value\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v2\"},{\"name\":\"AWS_ACCESS_KEY_ID\",\"value\":\"fake\"},{\"name\":\"AWS_SECRET_ACCESS_KEY\",\"value\":\"fake\"},{\"name\":\"AWS_REGION\",\"value\":\"notaregion\"},{\"name\":\"AWS_OWNER_ACCOUNT\",\"value\":\"fake\"},{\"name\":\"SQS_ENDPOINT\",\"value\":\"http://sqs:9324\"},{\"name\":\"RELEASE_UDPATE_SQS_QUEUE\",\"value\":\"preflight-checker\"},{\"name\":\"LICENSE_UDPATE_SQS_QUEUE\",\"value\":\"search-builder\"},{\"name\":\"AWS_SQS_MAIL_QUEUENAME\",\"value\":\"mail-dev\"},{\"name\":\"AWS_SQS_AIRGAP_QUEUENAME\",\"value\":\"airgap-dev\"},{\"name\":\"AWS_SQS_LICENSEAGGREGATOR_QUEUENAME\",\"value\":\"licenseaggregator-dev\"},{\"name\":\"INTEGRATION_API_SQS_QUEUE_NAME\",\"value\":\"integration-api-dev\"},{\"name\":\"BUGSNAG_ENV\",\"value\":\"dev\"},{\"name\":\"VENDOR_REGISTY_HOST\",\"value\":\"https://registry:10443\"},{\"name\":\"VENDOR_REGISTY_ENDPOINT\",\"value\":\"registry:10443\"},{\"name\":\"LOG_LEVEL\",\"value\":\"debug\"},{\"name\":\"ENVIRONMENT\",\"value\":\"dev\"},{\"name\":\"ALLOW_INSECURE_REGISTRY\",\"value\":\"true\"},{\"name\":\"GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api:3000/graphql\"},{\"name\":\"PREM_GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api-prem:3000/graphql\"},{\"name\":\"GRAPHQL_ENDPOINT\",\"value\":\"http://awesome-api:3000/graphql\"},{\"name\":\"TITLED_ENDPOINT\",\"value\":\"http://gatekeeping-api:3000\"},{\"name\":\"MYSQL_HOST\",\"value\":\"mysql\"},{\"name\":\"MYSQL_USER\",\"value\":\"mysqluser\"},{\"name\":\"MYSQL_PASSWORD\",\"value\":\"password\"},{\"name\":\"MYSQL_PORT\",\"value\":\"3306\"},{\"name\":\"MYSQL_DATABASE\",\"value\":\"mysqldb\"},{\"name\":\"AUDITOR_API_HOST\",\"value\":\"http://api.auditor.svc.cluster.local:3000/auditlog\"},{\"name\":\"AUDITOR_TOKEN\",\"value\":\"dev\"},{\"name\":\"AUDITOR_PROJECTID\",\"value\":\"dev\"},{\"name\":\"PROJECT_NAME\",\"value\":\"cranky-api\"}],\"image\":\"localhost:32000/cranky-api:e9a281f7-dirty@sha256:f6bbd155a2e7325ea93dbc48848eeba7b0e612b8849676df91e647f0d86ece03\",\"imagePullPolicy\":\"IfNotPresent\",\"livenessProbe\":{\"failureThreshold\":2,\"httpGet\":{\"path\":\"/healthz\",\"port\":8005,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":30,\"periodSeconds\":15,\"timeoutSeconds\":1},\"name\":\"cranky-api\",\"ports\":[{\"containerPort\":8005,\"name\":\"cranky-api\"}],\"readinessProbe\":{\"failureThreshold\":3,\"httpGet\":{\"path\":\"/healthz\",\"port\":8005,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":5,\"periodSeconds\":2,\"successThreshold\":2,\"timeoutSeconds\":1},\"workingDir\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api\"}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "cranky-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				"skaffold.dev/builder": "local",
				"skaffold.dev/cleanup": "true",
				"skaffold.dev/deployer": "kustomize",
				"skaffold.dev/docker-api-version": "1.39",
				"skaffold.dev/tag-policy": "git-commit",
				"skaffold.dev/tail": "true"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "cranky-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "cranky-api",
					"image": "localhost:32000/cranky-api:e9a281f7-dirty@sha256:f6bbd155a2e7325ea93dbc48848eeba7b0e612b8849676df91e647f0d86ece03",
					"workingDir": "/go/src/github.com/somebigbankhq/vandam/cranky-api",
					"ports": [
					  {
						"name": "cranky-api",
						"containerPort": 8005,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "ELASTICSEARCH_NODES",
						"value": "http://esuser:thisisnotasecret@elasticsearch:9200"
					  },
					  {
						"name": "MAX_LOGIN_ATTEMPTS",
						"value": "7"
					  },
					  {
						"name": "LICENSE_SIGNING_KEY"
					  },
					  {
						"name": "JWT_SIGNING_KEY",
						"value": "jwt-signing-key"
					  },
					  {
						"name": "GITHUB_CLIENT_ID",
						"value": "9a92961100c3bd14b991"
					  },
					  {
						"name": "GITHUB_CLIENT_SECRET",
						"value": "not-a-secret"
					  },
					  {
						"name": "LICENSE_API_ENDPOINT",
						"value": "http://boring-api:3000"
					  },
					  {
						"name": "vendor_assets_bucket",
						"value": "replicaetd-vendor-assets-dev"
					  },
					  {
						"name": "vendor_web_host",
						"value": "http://localhost:8080"
					  },
					  {
						"name": "download_web_host",
						"value": "http://localhost:8080"
					  },
					  {
						"name": "SPEC_V1_PATH",
						"value": "/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v1"
					  },
					  {
						"name": "SPEC_V2_PATH",
						"value": "/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v2"
					  },
					  {
						"name": "AWS_ACCESS_KEY_ID",
						"value": "fake"
					  },
					  {
						"name": "AWS_SECRET_ACCESS_KEY",
						"value": "fake"
					  },
					  {
						"name": "AWS_REGION",
						"value": "notaregion"
					  },
					  {
						"name": "AWS_OWNER_ACCOUNT",
						"value": "fake"
					  },
					  {
						"name": "SQS_ENDPOINT",
						"value": "http://sqs:9324"
					  },
					  {
						"name": "RELEASE_UDPATE_SQS_QUEUE",
						"value": "preflight-checker"
					  },
					  {
						"name": "LICENSE_UDPATE_SQS_QUEUE",
						"value": "search-builder"
					  },
					  {
						"name": "AWS_SQS_MAIL_QUEUENAME",
						"value": "mail-dev"
					  },
					  {
						"name": "AWS_SQS_AIRGAP_QUEUENAME",
						"value": "airgap-dev"
					  },
					  {
						"name": "AWS_SQS_LICENSEAGGREGATOR_QUEUENAME",
						"value": "licenseaggregator-dev"
					  },
					  {
						"name": "INTEGRATION_API_SQS_QUEUE_NAME",
						"value": "integration-api-dev"
					  },
					  {
						"name": "BUGSNAG_ENV",
						"value": "dev"
					  },
					  {
						"name": "VENDOR_REGISTY_HOST",
						"value": "https://registry:10443"
					  },
					  {
						"name": "VENDOR_REGISTY_ENDPOINT",
						"value": "registry:10443"
					  },
					  {
						"name": "LOG_LEVEL",
						"value": "debug"
					  },
					  {
						"name": "ENVIRONMENT",
						"value": "dev"
					  },
					  {
						"name": "ALLOW_INSECURE_REGISTRY",
						"value": "true"
					  },
					  {
						"name": "GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api:3000/graphql"
					  },
					  {
						"name": "PREM_GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api-prem:3000/graphql"
					  },
					  {
						"name": "GRAPHQL_ENDPOINT",
						"value": "http://awesome-api:3000/graphql"
					  },
					  {
						"name": "TITLED_ENDPOINT",
						"value": "http://gatekeeping-api:3000"
					  },
					  {
						"name": "MYSQL_HOST",
						"value": "mysql"
					  },
					  {
						"name": "MYSQL_USER",
						"value": "mysqluser"
					  },
					  {
						"name": "MYSQL_PASSWORD",
						"value": "password"
					  },
					  {
						"name": "MYSQL_PORT",
						"value": "3306"
					  },
					  {
						"name": "MYSQL_DATABASE",
						"value": "mysqldb"
					  },
					  {
						"name": "AUDITOR_API_HOST",
						"value": "http://api.auditor.svc.cluster.local:3000/auditlog"
					  },
					  {
						"name": "AUDITOR_TOKEN",
						"value": "dev"
					  },
					  {
						"name": "AUDITOR_PROJECTID",
						"value": "dev"
					  },
					  {
						"name": "PROJECT_NAME",
						"value": "cranky-api"
					  }
					],
					"resources": {},
					"livenessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 8005,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 30,
					  "timeoutSeconds": 1,
					  "periodSeconds": 15,
					  "successThreshold": 1,
					  "failureThreshold": 2
					},
					"readinessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 8005,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 5,
					  "timeoutSeconds": 1,
					  "periodSeconds": 2,
					  "successThreshold": 2,
					  "failureThreshold": 3
					},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:34Z",
				"lastTransitionTime": "2019-07-15T21:15:34Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "front-end",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/front-end",
			"uid": "aeabeecb-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197433",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"front-end\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"front-end\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"front-end\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"affinity\":{\"podAntiAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"podAffinityTerm\":{\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"front-end\"]}]},\"topologyKey\":\"kubernetes.io/hostname\"},\"weight\":2}]}},\"containers\":[{\"image\":\"localhost:32000/front-end:e9a281f7-dirty@sha256:382ed0992abac7e0d7b77ea6a43f0bed9fd5217dcf8550c2433db7037007636c\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"front-end\",\"ports\":[{\"containerPort\":8080,\"name\":\"front-end\"}]}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "front-end"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "front-end",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "front-end",
					"image": "localhost:32000/front-end:e9a281f7-dirty@sha256:382ed0992abac7e0d7b77ea6a43f0bed9fd5217dcf8550c2433db7037007636c",
					"ports": [
					  {
						"name": "front-end",
						"containerPort": 8080,
						"protocol": "TCP"
					  }
					],
					"resources": {},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"affinity": {
				  "podAntiAffinity": {
					"preferredDuringSchedulingIgnoredDuringExecution": [
					  {
						"weight": 2,
						"podAffinityTerm": {
						  "labelSelector": {
							"matchExpressions": [
							  {
								"key": "app",
								"operator": "In",
								"values": [
								  "front-end"
								]
							  }
							]
						  },
						  "topologyKey": "kubernetes.io/hostname"
						}
					  }
					]
				  }
				},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": "25%",
				"maxSurge": "25%"
			  }
			},
			"revisionHistoryLimit": 10,
			"progressDeadlineSeconds": 600
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:41Z",
				"lastTransitionTime": "2019-07-15T21:15:41Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  },
			  {
				"type": "Progressing",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:41Z",
				"lastTransitionTime": "2019-07-15T21:15:33Z",
				"reason": "NewReplicaSetAvailable",
				"message": "ReplicaSet \"front-end-54cf6bb649\" has successfully progressed."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "lazy-api",
			"namespace": "default",
			"selfLink": "/api/v1/namespaces/default/services/lazy-api",
			"uid": "aea94ce2-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197269",
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app": "lazy-api",
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"labels\":{\"app\":\"lazy-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"lazy-api\",\"namespace\":\"default\"},\"spec\":{\"ports\":[{\"name\":\"lazy-api\",\"nodePort\":30005,\"port\":8005,\"targetPort\":\"lazy-api\"}],\"selector\":{\"app\":\"lazy-api\"},\"type\":\"NodePort\"}}\n"
			}
		  },
		  "spec": {
			"ports": [
			  {
				"name": "lazy-api",
				"protocol": "TCP",
				"port": 8005,
				"targetPort": "lazy-api",
				"nodePort": 30005
			  }
			],
			"selector": {
			  "app": "lazy-api"
			},
			"type": "NodePort",
			"sessionAffinity": "None",
			"externalTrafficPolicy": "Cluster"
		  },
		  "status": {
			"loadBalancer": {}
		  }
		},
		{
			"auth_dump": [
				{
					"entity": "osd.0",
					"key": "ABCxyzABCxyz/foo/bar123xyz/BAZAABBCCDD==",
					"caps": {
						"mgr": "allow profile osd",
						"mon": "allow profile osd",
						"osd": "allow *"
					}
				},
				{
					"entity": "client.admin",
					"key": "ABCxyzABCxyz/foo/bar123xyz/BAZAABBCCDD==",
					"caps": {
						"mds": "allow *",
						"mgr": "allow *",
						"mon": "allow *",
						"osd": "allow *"
					}
				},
				{
					"entity": "client.bootstrap-mds",
					"key": "ABCxyzABCxyz/foo/bar123xyz/BAZAABBCCDD==",
					"caps": {
						"mon": "allow profile bootstrap-mds"
					}
				},
				{
					"entity": "client.rgw.rook.ceph.store.a",
					"key": "ABCxyzABCxyz/foo/bar123xyz/BAZAABBCCDD==",
					"caps": {
						"mon": "allow rw",
						"osd": "allow rwx"
					}
				},
				{
					"entity": "mgr.a",
					"key": "ABCxyzABCxyz/foo/bar123xyz/BAZAABBCCDD==",
					"caps": {
						"mds": "allow *",
						"mon": "allow profile mgr",
						"osd": "allow *"
					}
				}
			]
		}
	  ]`

	expected := `[
		{
		  "metadata": {
			"name": "awesome-api",
			"namespace": "default",
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"awesome-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"awesome-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"JWT_SIGNING_KEY\",\"value\":\"jwt-signing-key\"},{\"name\":\"TSED_SUPPRESS_ACCESSLOG\",\"value\":\"1\"},{\"name\":\"PINO_LOG_PRETTY\",\"value\":\"1\"},{\"name\":\"PINO_LOG_LEVEL\",\"value\":\"debug\"},{\"name\":\"NODE_ENV\",\"value\":\"development\"},{\"name\":\"MYSQL_HOST\",\"value\":\"mysql\"},{\"name\":\"MYSQL_USER\",\"value\":\"***HIDDEN***\"},{\"name\":\"MYSQL_PASSWORD\",\"value\":\"***HIDDEN***\"},{\"name\":\"MYSQL_PORT\",\"value\":\"3306\"},{\"name\":\"MYSQL_DATABASE\",\"value\":\"***HIDDEN***\"},{\"name\":\"CUSTOMER_AVATAR_S3_BUCKET\",\"value\":\"products-dev-avatars-3\"},{\"name\":\"SUPPORT_BUNDLE_S3_BUCKET\",\"value\":\"products-dev-supportbundles\"},{\"name\":\"APP_RELEASE_S3_BUCKET\",\"value\":\"products-dev-releases\"},{\"name\":\"GRAPHQL_VENDOR_ENDPOINT\",\"value\":\"http://awesome-api:8013/graphql\"},{\"name\":\"GRAPHQL_PREM_ENDPOINT\",\"value\":\"http://awesome-api-prem:8033/graphql\"},{\"name\":\"ANALYZE_ENDPOINT\",\"value\":\"http://lazy-api:3000\"},{\"name\":\"GITHUB_CLIENT_ID\",\"value\":\"Iv1.64993a1aeb9575e0\"},{\"name\":\"GITHUB_PRIVATE_KEY_FILENAME\",\"value\":\"/secret-mounts/github-app-private-key--dev-only.pem\"},{\"name\":\"GITHUB_INTEGRATION_ID\",\"value\":\"7888\"},{\"name\":\"INSTALL_URL\",\"value\":\"http://localhost:8090\"},{\"name\":\"AWS_ACCESS_KEY_ID\",\"value\":\"***HIDDEN***\"},{\"name\":\"AWS_SECRET_ACCESS_KEY\",\"value\":\"***HIDDEN***\"},{\"name\":\"AWS_REGION\",\"value\":\"notaregion\"},{\"name\":\"AWS_OWNER_ACCOUNT\",\"value\":\"***HIDDEN***\"},{\"name\":\"S3_ENDPOINT\",\"value\":\"http://s3:4569\"},{\"name\":\"SERVER_MODE\",\"value\":\"vendor\"},{\"name\":\"NEW_RELIC_APP_NAME\",\"value\":\"awesome-api-vendor\"}],\"image\":\"localhost:32000/awesome-api:e9a281f7@sha256:6e988461ffce2bac3561234f736b9a504bfda1911fa6432b90e6bbb16f67f925\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"awesome-api\",\"ports\":[{\"containerPort\":3000,\"name\":\"awesome-api\"}],\"readinessProbe\":{\"failureThreshold\":3,\"httpGet\":{\"path\":\"/healthz\",\"port\":3000,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":2,\"periodSeconds\":2,\"successThreshold\":1,\"timeoutSeconds\":1}}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "awesome-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "awesome-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "awesome-api",
					"image": "localhost:32000/awesome-api:e9a281f7@sha256:6e988461ffce2bac3561234f736b9a504bfda1911fa6432b90e6bbb16f67f925",
					"ports": [
					  {
						"name": "awesome-api",
						"containerPort": 3000,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "DB_CONNECTION_STRING",
						"value": "***HIDDEN***:***HIDDEN***@tcp(***HIDDEN***:3309)/***HIDDEN***"
					  },
					  {
						"name": "POSTGRES_CONNECTION_STRING",
						"value": "Provider=PostgreSQL OLE DB Provider;Data Source=***HIDDEN***;location=***HIDDEN***;User ID=***HIDDEN***;password=***HIDDEN***;timeout=1000;"
					  },
					  {
						"name": "POSTGRES_CONNECTION_STRING_2",
						"value": "postgres://***HIDDEN***:***HIDDEN***@***HIDDEN***:5432/***HIDDEN***"
					  },
					  {
						"name": "MYSQL_CONNECTION_STRING",
						"value": "Server=***HIDDEN***;Database=***HIDDEN***;Uid=***HIDDEN***;Pwd=***HIDDEN***;UseProcedureBodies=False;"
					  },
					  {
						"name": "JWT_SIGNING_KEY",
						"value": "jwt-signing-key"
					  },
					  {
						"name": "TSED_SUPPRESS_ACCESSLOG",
						"value": "1"
					  },
					  {
						"name": "PINO_LOG_PRETTY",
						"value": "1"
					  },
					  {
						"name": "PINO_LOG_LEVEL",
						"value": "debug"
					  },
					  {
						"name": "NODE_ENV",
						"value": "development"
					  },
					  {
						"name": "MYSQL_HOST",
						"value": "mysql"
					  },
					  {
						"name": "MYSQL_USER",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "MYSQL_PASSWORD",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "MYSQL_PORT",
						"value": "3306"
					  },
					  {
						"name": "MYSQL_DATABASE",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "CUSTOMER_AVATAR_S3_BUCKET",
						"value": "products-dev-avatars-3"
					  },
					  {
						"name": "SUPPORT_BUNDLE_S3_BUCKET",
						"value": "products-dev-supportbundles"
					  },
					  {
						"name": "APP_RELEASE_S3_BUCKET",
						"value": "products-dev-releases"
					  },
					  {
						"name": "GRAPHQL_VENDOR_ENDPOINT",
						"value": "http://awesome-api:8013/graphql"
					  },
					  {
						"name": "GRAPHQL_PREM_ENDPOINT",
						"value": "http://awesome-api-prem:8033/graphql"
					  },
					  {
						"name": "ANALYZE_ENDPOINT",
						"value": "http://lazy-api:3000"
					  },
					  {
						"name": "GITHUB_CLIENT_ID",
						"value": "Iv1.64993a1aeb9575e0"
					  },
					  {
						"name": "GITHUB_PRIVATE_KEY_FILENAME",
						"value": "/secret-mounts/github-app-private-key--dev-only.pem"
					  },
					  {
						"name": "GITHUB_INTEGRATION_ID",
						"value": "7888"
					  },
					  {
						"name": "INSTALL_URL",
						"value": "http://localhost:8090"
					  },
					  {
						"name": "AWS_ACCESS_KEY_ID",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AWS_SECRET_ACCESS_KEY",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AWS_REGION",
						"value": "notaregion"
					  },
					  {
						"name": "AWS_OWNER_ACCOUNT",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "S3_ENDPOINT",
						"value": "http://s3:4569"
					  },
					  {
						"name": "SERVER_MODE",
						"value": "vendor"
					  },
					  {
						"name": "NEW_RELIC_APP_NAME",
						"value": "awesome-api-vendor"
					  }
					],
					"resources": {},
					"readinessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 3000,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 2,
					  "timeoutSeconds": 1,
					  "periodSeconds": 2,
					  "successThreshold": 1,
					  "failureThreshold": 3
					},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:33Z",
				"lastTransitionTime": "2019-07-15T21:15:33Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "gatekeeping-api",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/gatekeeping-api",
			"uid": "aeb84dc6-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197507",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"gatekeeping-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"gatekeeping-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"args\":[\"serve\"],\"env\":[{\"name\":\"GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api-prem:3000/graphql\"}],\"image\":\"somebigbank/titled\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"gatekeeping-api\",\"ports\":[{\"containerPort\":3000,\"name\":\"gatekeeping-api\"}]}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "gatekeeping-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				"skaffold.dev/builder": "local",
				"skaffold.dev/cleanup": "true",
				"skaffold.dev/deployer": "kustomize",
				"skaffold.dev/docker-api-version": "1.39",
				"skaffold.dev/tag-policy": "git-commit",
				"skaffold.dev/tail": "true"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "gatekeeping-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "gatekeeping-api",
					"image": "somebigbank/titled",
					"args": [
					  "serve"
					],
					"ports": [
					  {
						"name": "gatekeeping-api",
						"containerPort": 3000,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api-prem:3000/graphql"
					  }
					],
					"resources": {},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:34Z",
				"lastTransitionTime": "2019-07-15T21:15:34Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "cranky-api",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/cranky-api",
			"uid": "aebb750c-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197557",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"cranky-api\",\"namespace\":\"default\"},\"spec\":{\"template\":{\"metadata\":{\"labels\":{\"app\":\"cranky-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"ELASTICSEARCH_NODES\",\"value\":\"http://***HIDDEN***:***HIDDEN***@elasticsearch:9200\"},{\"name\":\"MAX_LOGIN_ATTEMPTS\",\"value\":\"7\"},{\"name\":\"LICENSE_SIGNING_KEY\",\"value\":\"\"},{\"name\":\"JWT_SIGNING_KEY\",\"value\":\"jwt-signing-key\"},{\"name\":\"GITHUB_CLIENT_ID\",\"value\":\"9a92961100c3bd14b991\"},{\"name\":\"GITHUB_CLIENT_SECRET\",\"value\":\"not-a-secret\"},{\"name\":\"LICENSE_API_ENDPOINT\",\"value\":\"http://boring-api:3000\"},{\"name\":\"vendor_assets_bucket\",\"value\":\"replicaetd-vendor-assets-dev\"},{\"name\":\"vendor_web_host\",\"value\":\"http://localhost:8080\"},{\"name\":\"download_web_host\",\"value\":\"http://localhost:8080\"},{\"name\":\"SPEC_V1_PATH\",\"value\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v1\"},{\"name\":\"SPEC_V2_PATH\",\"value\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v2\"},{\"name\":\"AWS_ACCESS_KEY_ID\",\"value\":\"***HIDDEN***\"},{\"name\":\"AWS_SECRET_ACCESS_KEY\",\"value\":\"***HIDDEN***\"},{\"name\":\"AWS_REGION\",\"value\":\"notaregion\"},{\"name\":\"AWS_OWNER_ACCOUNT\",\"value\":\"***HIDDEN***\"},{\"name\":\"SQS_ENDPOINT\",\"value\":\"http://sqs:9324\"},{\"name\":\"RELEASE_UDPATE_SQS_QUEUE\",\"value\":\"preflight-checker\"},{\"name\":\"LICENSE_UDPATE_SQS_QUEUE\",\"value\":\"search-builder\"},{\"name\":\"AWS_SQS_MAIL_QUEUENAME\",\"value\":\"mail-dev\"},{\"name\":\"AWS_SQS_AIRGAP_QUEUENAME\",\"value\":\"airgap-dev\"},{\"name\":\"AWS_SQS_LICENSEAGGREGATOR_QUEUENAME\",\"value\":\"licenseaggregator-dev\"},{\"name\":\"INTEGRATION_API_SQS_QUEUE_NAME\",\"value\":\"integration-api-dev\"},{\"name\":\"BUGSNAG_ENV\",\"value\":\"dev\"},{\"name\":\"VENDOR_REGISTY_HOST\",\"value\":\"https://registry:10443\"},{\"name\":\"VENDOR_REGISTY_ENDPOINT\",\"value\":\"registry:10443\"},{\"name\":\"LOG_LEVEL\",\"value\":\"debug\"},{\"name\":\"ENVIRONMENT\",\"value\":\"dev\"},{\"name\":\"ALLOW_INSECURE_REGISTRY\",\"value\":\"true\"},{\"name\":\"GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api:3000/graphql\"},{\"name\":\"PREM_GRAPHQL_API_ADDRESS\",\"value\":\"http://awesome-api-prem:3000/graphql\"},{\"name\":\"GRAPHQL_ENDPOINT\",\"value\":\"http://awesome-api:3000/graphql\"},{\"name\":\"TITLED_ENDPOINT\",\"value\":\"http://gatekeeping-api:3000\"},{\"name\":\"MYSQL_HOST\",\"value\":\"mysql\"},{\"name\":\"MYSQL_USER\",\"value\":\"***HIDDEN***\"},{\"name\":\"MYSQL_PASSWORD\",\"value\":\"***HIDDEN***\"},{\"name\":\"MYSQL_PORT\",\"value\":\"3306\"},{\"name\":\"MYSQL_DATABASE\",\"value\":\"***HIDDEN***\"},{\"name\":\"AUDITOR_API_HOST\",\"value\":\"http://api.auditor.svc.cluster.local:3000/auditlog\"},{\"name\":\"AUDITOR_TOKEN\",\"value\":\"***HIDDEN***\"},{\"name\":\"AUDITOR_PROJECTID\",\"value\":\"dev\"},{\"name\":\"PROJECT_NAME\",\"value\":\"cranky-api\"}],\"image\":\"localhost:32000/cranky-api:e9a281f7-dirty@sha256:f6bbd155a2e7325ea93dbc48848eeba7b0e612b8849676df91e647f0d86ece03\",\"imagePullPolicy\":\"IfNotPresent\",\"livenessProbe\":{\"failureThreshold\":2,\"httpGet\":{\"path\":\"/healthz\",\"port\":8005,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":30,\"periodSeconds\":15,\"timeoutSeconds\":1},\"name\":\"cranky-api\",\"ports\":[{\"containerPort\":8005,\"name\":\"cranky-api\"}],\"readinessProbe\":{\"failureThreshold\":3,\"httpGet\":{\"path\":\"/healthz\",\"port\":8005,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":5,\"periodSeconds\":2,\"successThreshold\":2,\"timeoutSeconds\":1},\"workingDir\":\"/go/src/github.com/somebigbankhq/vandam/cranky-api\"}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "cranky-api",
				"app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				"skaffold.dev/builder": "local",
				"skaffold.dev/cleanup": "true",
				"skaffold.dev/deployer": "kustomize",
				"skaffold.dev/docker-api-version": "1.39",
				"skaffold.dev/tag-policy": "git-commit",
				"skaffold.dev/tail": "true"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "cranky-api",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "cranky-api",
					"image": "localhost:32000/cranky-api:e9a281f7-dirty@sha256:f6bbd155a2e7325ea93dbc48848eeba7b0e612b8849676df91e647f0d86ece03",
					"workingDir": "/go/src/github.com/somebigbankhq/vandam/cranky-api",
					"ports": [
					  {
						"name": "cranky-api",
						"containerPort": 8005,
						"protocol": "TCP"
					  }
					],
					"env": [
					  {
						"name": "ELASTICSEARCH_NODES",
						"value": "http://***HIDDEN***:***HIDDEN***@elasticsearch:9200"
					  },
					  {
						"name": "MAX_LOGIN_ATTEMPTS",
						"value": "7"
					  },
					  {
						"name": "LICENSE_SIGNING_KEY"
					  },
					  {
						"name": "JWT_SIGNING_KEY",
						"value": "jwt-signing-key"
					  },
					  {
						"name": "GITHUB_CLIENT_ID",
						"value": "9a92961100c3bd14b991"
					  },
					  {
						"name": "GITHUB_CLIENT_SECRET",
						"value": "not-a-secret"
					  },
					  {
						"name": "LICENSE_API_ENDPOINT",
						"value": "http://boring-api:3000"
					  },
					  {
						"name": "vendor_assets_bucket",
						"value": "replicaetd-vendor-assets-dev"
					  },
					  {
						"name": "vendor_web_host",
						"value": "http://localhost:8080"
					  },
					  {
						"name": "download_web_host",
						"value": "http://localhost:8080"
					  },
					  {
						"name": "SPEC_V1_PATH",
						"value": "/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v1"
					  },
					  {
						"name": "SPEC_V2_PATH",
						"value": "/go/src/github.com/somebigbankhq/vandam/cranky-api/spec/v2"
					  },
					  {
						"name": "AWS_ACCESS_KEY_ID",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AWS_SECRET_ACCESS_KEY",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AWS_REGION",
						"value": "notaregion"
					  },
					  {
						"name": "AWS_OWNER_ACCOUNT",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "SQS_ENDPOINT",
						"value": "http://sqs:9324"
					  },
					  {
						"name": "RELEASE_UDPATE_SQS_QUEUE",
						"value": "preflight-checker"
					  },
					  {
						"name": "LICENSE_UDPATE_SQS_QUEUE",
						"value": "search-builder"
					  },
					  {
						"name": "AWS_SQS_MAIL_QUEUENAME",
						"value": "mail-dev"
					  },
					  {
						"name": "AWS_SQS_AIRGAP_QUEUENAME",
						"value": "airgap-dev"
					  },
					  {
						"name": "AWS_SQS_LICENSEAGGREGATOR_QUEUENAME",
						"value": "licenseaggregator-dev"
					  },
					  {
						"name": "INTEGRATION_API_SQS_QUEUE_NAME",
						"value": "integration-api-dev"
					  },
					  {
						"name": "BUGSNAG_ENV",
						"value": "dev"
					  },
					  {
						"name": "VENDOR_REGISTY_HOST",
						"value": "https://registry:10443"
					  },
					  {
						"name": "VENDOR_REGISTY_ENDPOINT",
						"value": "registry:10443"
					  },
					  {
						"name": "LOG_LEVEL",
						"value": "debug"
					  },
					  {
						"name": "ENVIRONMENT",
						"value": "dev"
					  },
					  {
						"name": "ALLOW_INSECURE_REGISTRY",
						"value": "true"
					  },
					  {
						"name": "GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api:3000/graphql"
					  },
					  {
						"name": "PREM_GRAPHQL_API_ADDRESS",
						"value": "http://awesome-api-prem:3000/graphql"
					  },
					  {
						"name": "GRAPHQL_ENDPOINT",
						"value": "http://awesome-api:3000/graphql"
					  },
					  {
						"name": "TITLED_ENDPOINT",
						"value": "http://gatekeeping-api:3000"
					  },
					  {
						"name": "MYSQL_HOST",
						"value": "mysql"
					  },
					  {
						"name": "MYSQL_USER",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "MYSQL_PASSWORD",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "MYSQL_PORT",
						"value": "3306"
					  },
					  {
						"name": "MYSQL_DATABASE",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AUDITOR_API_HOST",
						"value": "http://api.auditor.svc.cluster.local:3000/auditlog"
					  },
					  {
						"name": "AUDITOR_TOKEN",
						"value": "***HIDDEN***"
					  },
					  {
						"name": "AUDITOR_PROJECTID",
						"value": "dev"
					  },
					  {
						"name": "PROJECT_NAME",
						"value": "cranky-api"
					  }
					],
					"resources": {},
					"livenessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 8005,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 30,
					  "timeoutSeconds": 1,
					  "periodSeconds": 15,
					  "successThreshold": 1,
					  "failureThreshold": 2
					},
					"readinessProbe": {
					  "httpGet": {
						"path": "/healthz",
						"port": 8005,
						"scheme": "HTTP"
					  },
					  "initialDelaySeconds": 5,
					  "timeoutSeconds": 1,
					  "periodSeconds": 2,
					  "successThreshold": 2,
					  "failureThreshold": 3
					},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": 1,
				"maxSurge": 1
			  }
			},
			"revisionHistoryLimit": 2147483647,
			"progressDeadlineSeconds": 2147483647
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:34Z",
				"lastTransitionTime": "2019-07-15T21:15:34Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "front-end",
			"namespace": "default",
			"selfLink": "/apis/apps/v1/namespaces/default/deployments/front-end",
			"uid": "aeabeecb-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197433",
			"generation": 1,
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "deployment.kubernetes.io/revision": "1",
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"front-end\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"front-end\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"front-end\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"affinity\":{\"podAntiAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"podAffinityTerm\":{\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"front-end\"]}]},\"topologyKey\":\"kubernetes.io/hostname\"},\"weight\":2}]}},\"containers\":[{\"image\":\"localhost:32000/front-end:e9a281f7-dirty@sha256:382ed0992abac7e0d7b77ea6a43f0bed9fd5217dcf8550c2433db7037007636c\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"front-end\",\"ports\":[{\"containerPort\":8080,\"name\":\"front-end\"}]}]}}}}\n"
			}
		  },
		  "spec": {
			"replicas": 1,
			"selector": {
			  "matchLabels": {
				"app": "front-end"
			  }
			},
			"template": {
			  "metadata": {
				"creationTimestamp": null,
				"labels": {
				  "app": "front-end",
				  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
				  "skaffold.dev/builder": "local",
				  "skaffold.dev/cleanup": "true",
				  "skaffold.dev/deployer": "kustomize",
				  "skaffold.dev/docker-api-version": "1.39",
				  "skaffold.dev/tag-policy": "git-commit",
				  "skaffold.dev/tail": "true"
				}
			  },
			  "spec": {
				"containers": [
				  {
					"name": "front-end",
					"image": "localhost:32000/front-end:e9a281f7-dirty@sha256:382ed0992abac7e0d7b77ea6a43f0bed9fd5217dcf8550c2433db7037007636c",
					"ports": [
					  {
						"name": "front-end",
						"containerPort": 8080,
						"protocol": "TCP"
					  }
					],
					"resources": {},
					"terminationMessagePath": "/dev/termination-log",
					"terminationMessagePolicy": "File",
					"imagePullPolicy": "IfNotPresent"
				  }
				],
				"restartPolicy": "Always",
				"terminationGracePeriodSeconds": 30,
				"dnsPolicy": "ClusterFirst",
				"securityContext": {},
				"affinity": {
				  "podAntiAffinity": {
					"preferredDuringSchedulingIgnoredDuringExecution": [
					  {
						"weight": 2,
						"podAffinityTerm": {
						  "labelSelector": {
							"matchExpressions": [
							  {
								"key": "app",
								"operator": "In",
								"values": [
								  "front-end"
								]
							  }
							]
						  },
						  "topologyKey": "kubernetes.io/hostname"
						}
					  }
					]
				  }
				},
				"schedulerName": "default-scheduler"
			  }
			},
			"strategy": {
			  "type": "RollingUpdate",
			  "rollingUpdate": {
				"maxUnavailable": "25%",
				"maxSurge": "25%"
			  }
			},
			"revisionHistoryLimit": 10,
			"progressDeadlineSeconds": 600
		  },
		  "status": {
			"observedGeneration": 1,
			"replicas": 1,
			"updatedReplicas": 1,
			"readyReplicas": 1,
			"availableReplicas": 1,
			"conditions": [
			  {
				"type": "Available",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:41Z",
				"lastTransitionTime": "2019-07-15T21:15:41Z",
				"reason": "MinimumReplicasAvailable",
				"message": "Deployment has minimum availability."
			  },
			  {
				"type": "Progressing",
				"status": "True",
				"lastUpdateTime": "2019-07-15T21:15:41Z",
				"lastTransitionTime": "2019-07-15T21:15:33Z",
				"reason": "NewReplicaSetAvailable",
				"message": "ReplicaSet \"front-end-54cf6bb649\" has successfully progressed."
			  }
			]
		  }
		},
		{
		  "metadata": {
			"name": "lazy-api",
			"namespace": "default",
			"selfLink": "/api/v1/namespaces/default/services/lazy-api",
			"uid": "aea94ce2-a745-11e9-8ebd-42010aa8001f",
			"resourceVersion": "7197269",
			"creationTimestamp": "2019-07-15T21:15:33Z",
			"labels": {
			  "app": "lazy-api",
			  "app.kubernetes.io/managed-by": "skaffold-v0.32.0",
			  "skaffold.dev/builder": "local",
			  "skaffold.dev/cleanup": "true",
			  "skaffold.dev/deployer": "kustomize",
			  "skaffold.dev/docker-api-version": "1.39",
			  "skaffold.dev/tag-policy": "git-commit",
			  "skaffold.dev/tail": "true"
			},
			"annotations": {
			  "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"labels\":{\"app\":\"lazy-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.32.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.39\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"lazy-api\",\"namespace\":\"default\"},\"spec\":{\"ports\":[{\"name\":\"lazy-api\",\"nodePort\":30005,\"port\":8005,\"targetPort\":\"lazy-api\"}],\"selector\":{\"app\":\"lazy-api\"},\"type\":\"NodePort\"}}\n"
			}
		  },
		  "spec": {
			"ports": [
			  {
				"name": "lazy-api",
				"protocol": "TCP",
				"port": 8005,
				"targetPort": "lazy-api",
				"nodePort": 30005
			  }
			],
			"selector": {
			  "app": "lazy-api"
			},
			"type": "NodePort",
			"sessionAffinity": "None",
			"externalTrafficPolicy": "Cluster"
		  },
		  "status": {
			"loadBalancer": {}
		  }
		},
		{
			"auth_dump": [
				{
					"entity": "osd.0",
					"key": "***HIDDEN***",
					"caps": {
						"mgr": "allow profile osd",
						"mon": "allow profile osd",
						"osd": "allow *"
					}
				},
				{
					"entity": "client.admin",
					"key": "***HIDDEN***",
					"caps": {
						"mds": "allow *",
						"mgr": "allow *",
						"mon": "allow *",
						"osd": "allow *"
					}
				},
				{
					"entity": "client.bootstrap-mds",
					"key": "***HIDDEN***",
					"caps": {
						"mon": "allow profile bootstrap-mds"
					}
				},
				{
					"entity": "client.rgw.rook.ceph.store.a",
					"key": "***HIDDEN***",
					"caps": {
						"mon": "allow rw",
						"osd": "allow rwx"
					}
				},
				{
					"entity": "mgr.a",
					"key": "***HIDDEN***",
					"caps": {
						"mds": "allow *",
						"mon": "allow profile mgr",
						"osd": "allow *"
					}
				}
			]
		}
	  ]`

	wantRedactionsLen := 43
	wantRedactionsCount := 25

	t.Run("test default redactors", func(t *testing.T) {
		req := require.New(t)
		ResetRedactionList() // Clean up before test
		redactors, err := getRedactors("testpath")
		req.NoError(err)

		nextReader := io.Reader(strings.NewReader(original))
		for _, r := range redactors {
			nextReader = r.Redact(nextReader, "")
		}

		redacted, err := ioutil.ReadAll(nextReader)
		req.NoError(err)

		req.JSONEq(expected, string(redacted))

		actualRedactions := GetRedactionList()
		ResetRedactionList()
		req.Len(actualRedactions.ByFile["testpath"], wantRedactionsLen)
		req.Len(actualRedactions.ByRedactor, wantRedactionsCount)
		ResetRedactionList()
	})
}

func Test_redactMatchesPath(t *testing.T) {
	type args struct {
		path   string
		redact *troubleshootv1beta2.Redact
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "literal path",
			args: args{
				path: "/my/test/path",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/path",
						Files: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "no path",
			args: args{
				path: "/my/test/path",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "",
						Files: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "wrong literal path",
			args: args{
				path: "/my/test/path",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/path/two",
						Files: nil,
					},
				},
			},
			want: false,
		},
		{
			name: "path with glob",
			args: args{
				path: "/my/test/path/two",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/path/*",
						Files: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "path with glob in middle",
			args: args{
				path: "/my/test/path/two",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/*/*",
						Files: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "multiple paths",
			args: args{
				path: "/my/test/path/two",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File: "",
						Files: []string{
							"/not/the/path",
							"/my/test/*/*",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "double glob matching separator",
			args: args{
				path: "/my/test/path/two",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/**",
						Files: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "single glob does NOT match separator",
			args: args{
				path: "/my/test/path/two",
				redact: &troubleshootv1beta2.Redact{
					FileSelector: troubleshootv1beta2.FileSelector{
						File:  "/my/test/*",
						Files: nil,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got, err := redactMatchesPath(tt.args.path, tt.args.redact)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
