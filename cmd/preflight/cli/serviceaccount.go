package cli

import (
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createServiceAccount(preflight troubleshootv1beta1.Preflight, namespace string, clientset *kubernetes.Clientset) (string, error) {
	name := fmt.Sprintf("preflight-%s", preflight.Name)

	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets: []corev1.ObjectReference{
			{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       name,
				Namespace:  namespace,
			},
		},
	}
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
	if err != nil {
		return "", err
	}

	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"namespaces",
					"pods",
					"services",
					"secrets",
				},
				Verbs: metav1.Verbs{"list"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     metav1.Verbs{"list"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"ingresses"},
				Verbs:     metav1.Verbs{"list"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     metav1.Verbs{"list"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     metav1.Verbs{"list"},
			},
		},
	}
	_, err = clientset.RbacV1().ClusterRoles().Create(&role)
	if err != nil {
		return "", err
	}

	roleBinding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
	}
	_, err = clientset.RbacV1().ClusterRoleBindings().Create(&roleBinding)
	if err != nil {
		return "", err
	}

	return name, nil
}

func removeServiceAccount(name string, namespace string, clientset *kubernetes.Clientset) error {
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
		return err
	}


	return nil
}
