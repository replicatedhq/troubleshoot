package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
)

func (c *Run) GetImage() string {
	return c.Image
}

func (c *Run) SetImage(image string) {
	c.Image = image
}

func (c *Run) GetImagePullSecret() *ImagePullSecrets {
	return c.ImagePullSecret
}

func (c *Run) SetImagePullSecret(secrets *ImagePullSecrets) {
	c.ImagePullSecret = secrets
}

func (c *Run) GetNamespace() string {
	return c.Namespace
}

func (c *CopyFromHost) GetImage() string {
	return c.Image
}

func (c *CopyFromHost) SetImage(image string) {
	c.Image = image
}

func (c *CopyFromHost) GetImagePullSecret() *ImagePullSecrets {
	return c.ImagePullSecret
}

func (c *CopyFromHost) SetImagePullSecret(secrets *ImagePullSecrets) {
	c.ImagePullSecret = secrets
}

func (c *CopyFromHost) GetNamespace() string {
	return c.Namespace
}

func (c *Sysctl) GetImage() string {
	return c.Image
}

func (c *Sysctl) SetImage(image string) {
	c.Image = image
}

func (c *Sysctl) GetImagePullSecret() *ImagePullSecrets {
	return c.ImagePullSecret
}

func (c *Sysctl) SetImagePullSecret(secrets *ImagePullSecrets) {
	c.ImagePullSecret = secrets
}

func (c *Sysctl) GetNamespace() string {
	return c.Namespace
}

func (c *Collectd) GetImage() string {
	return c.Image
}

func (c *Collectd) SetImage(image string) {
	c.Image = image
}

func (c *Collectd) GetImagePullSecret() *ImagePullSecrets {
	return c.ImagePullSecret
}

func (c *Collectd) SetImagePullSecret(secrets *ImagePullSecrets) {
	c.ImagePullSecret = secrets
}

func (c *Collectd) GetNamespace() string {
	return c.Namespace
}

func (c *RunPod) GetPodSpec() corev1.PodSpec {
	return c.PodSpec
}

func (c *RunPod) SetPodSpec(podSpec corev1.PodSpec) {
	c.PodSpec = podSpec
}

func (c *RunPod) GetImagePullSecret() *ImagePullSecrets {
	return c.ImagePullSecret
}

func (c *RunPod) SetImagePullSecret(secrets *ImagePullSecrets) {
	c.ImagePullSecret = secrets
}

func (c *RunPod) GetNamespace() string {
	return c.Namespace
}
