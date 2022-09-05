/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"context"
	"time"

	v1beta1 "github.com/replicatedhq/troubleshoot/pkg/longhorn/apis/longhorn/v1beta1"
	scheme "github.com/replicatedhq/troubleshoot/pkg/longhorn/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SettingsGetter has a method to return a SettingInterface.
// A group's client should implement this interface.
type SettingsGetter interface {
	Settings(namespace string) SettingInterface
}

// SettingInterface has methods to work with Setting resources.
type SettingInterface interface {
	Create(ctx context.Context, setting *v1beta1.Setting, opts v1.CreateOptions) (*v1beta1.Setting, error)
	Update(ctx context.Context, setting *v1beta1.Setting, opts v1.UpdateOptions) (*v1beta1.Setting, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.Setting, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.SettingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Setting, err error)
	SettingExpansion
}

// settings implements SettingInterface
type settings struct {
	client rest.Interface
	ns     string
}

// newSettings returns a Settings
func newSettings(c *LonghornV1beta1Client, namespace string) *settings {
	return &settings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the setting, and returns the corresponding setting object, and an error if there is any.
func (c *settings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.Setting, err error) {
	result = &v1beta1.Setting{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("settings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Settings that match those selectors.
func (c *settings) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.SettingList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.SettingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("settings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested settings.
func (c *settings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("settings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a setting and creates it.  Returns the server's representation of the setting, and an error, if there is any.
func (c *settings) Create(ctx context.Context, setting *v1beta1.Setting, opts v1.CreateOptions) (result *v1beta1.Setting, err error) {
	result = &v1beta1.Setting{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("settings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(setting).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a setting and updates it. Returns the server's representation of the setting, and an error, if there is any.
func (c *settings) Update(ctx context.Context, setting *v1beta1.Setting, opts v1.UpdateOptions) (result *v1beta1.Setting, err error) {
	result = &v1beta1.Setting{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("settings").
		Name(setting.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(setting).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the setting and deletes it. Returns an error if one occurs.
func (c *settings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("settings").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *settings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("settings").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched setting.
func (c *settings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.Setting, err error) {
	result = &v1beta1.Setting{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("settings").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
