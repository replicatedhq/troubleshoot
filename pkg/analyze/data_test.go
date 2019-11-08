package analyzer

var collectedDeployments = `[
	{
	  "metadata": {
	    "name": "kotsadm-api",
	    "namespace": "default",
	    "selfLink": "/apis/apps/v1/namespaces/default/deployments/kotsadm-api",
	    "uid": "56526035-cd29-4d08-8375-291503b1a006",
	    "resourceVersion": "1583068",
	    "generation": 1,
	    "creationTimestamp": "2019-11-07T00:34:32Z",
	    "labels": {
	      "app.kubernetes.io/managed-by": "skaffold-v0.41.0"
	    },
	    "annotations": {
	      "deployment.kubernetes.io/revision": "1",
	      "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"kotsadm-api\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"kotsadm-api\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"kotsadm-api\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"affinity\":{\"podAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"podAffinityTerm\":{\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"ship-www\"]}]},\"topologyKey\":\"kubernetes.io/hostname\"},\"weight\":1}]},\"podAntiAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"podAffinityTerm\":{\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"kotsadm-api\"]}]},\"topologyKey\":\"kubernetes.io/hostname\"},\"weight\":2}]}},\"containers\":[{\"env\":[{\"name\":\"DEV_NAMESPACE\",\"value\":\"test\"},{\"name\":\"LOG_LEVEL\",\"value\":\"debug\"},{\"name\":\"SESSION_KEY\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"key\",\"name\":\"session\"}}},{\"name\":\"POSTGRES_URI\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"uri\",\"name\":\"ship-postgres\"}}},{\"name\":\"API_ENCRYPTION_KEY\",\"value\":\"IvWItkB8+ezMisPjSMBknT1PdKjBx7Xc/txZqOP8Y2Oe7+Jy\"},{\"name\":\"INIT_SERVER_URI\",\"value\":\"http://init-server:3000\"},{\"name\":\"WATCH_SERVER_URI\",\"value\":\"http://watch-server:3000\"},{\"name\":\"PINO_LOG_PRETTY\",\"value\":\"1\"},{\"name\":\"S3_BUCKET_NAME\",\"value\":\"shipbucket\"},{\"name\":\"AIRGAP_BUNDLE_S3_BUCKET\",\"value\":\"airgap\"},{\"name\":\"S3_ENDPOINT\",\"value\":\"http://kotsadm-s3.default.svc.cluster.local:4569/\"},{\"name\":\"S3_ACCESS_KEY_ID\",\"value\":\"***HIDDEN***\"},{\"name\":\"S3_SECRET_ACCESS_KEY\",\"value\":\"***HIDDEN***\"},{\"name\":\"S3_BUCKET_ENDPOINT\",\"value\":\"true\"},{\"name\":\"GITHUB_CLIENT_ID\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"client-id\",\"name\":\"github-app\"}}},{\"name\":\"GITHUB_CLIENT_SECRET\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"client-secret\",\"name\":\"github-app\"}}},{\"name\":\"GITHUB_INTEGRATION_ID\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"integration-id\",\"name\":\"github-app\"}}},{\"name\":\"GITHUB_PRIVATE_KEY_FILE\",\"value\":\"/keys/github/private-key.pem\"},{\"name\":\"SHIP_API_ENDPOINT\",\"value\":\"http://kotsadm-api.default.svc.cluster.local:3000\"},{\"name\":\"SHIP_API_ADVERTISE_ENDPOINT\",\"value\":\"http://localhost:30065\"},{\"name\":\"GRAPHQL_PREM_ENDPOINT\",\"value\":\"http://graphql-api-prem:3000/graphql\"},{\"name\":\"AUTO_CREATE_CLUSTER\",\"value\":\"1\"},{\"name\":\"AUTO_CREATE_CLUSTER_NAME\",\"value\":\"microk8s\"},{\"name\":\"AUTO_CREATE_CLUSTER_TOKEN\",\"value\":\"***HIDDEN***\"},{\"name\":\"ENABLE_SHIP\",\"value\":\"1\"},{\"name\":\"ENABLE_KOTS\",\"value\":\"1\"},{\"name\":\"ENABLE_KURL\",\"value\":\"1\"},{\"name\":\"POD_NAMESPACE\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"metadata.namespace\"}}}],\"image\":\"localhost:32000/kotsadm-api:v1.0.1-30-g8fa13e34-dirty@sha256:4a0ca1a2eae46472bd2d454f9e763a458ddd172689e179b17054262507bb4fc8\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"kotsadm-api\",\"ports\":[{\"containerPort\":3000,\"name\":\"http\"},{\"containerPort\":9229,\"name\":\"debug\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/healthz\",\"port\":3000},\"initialDelaySeconds\":2,\"periodSeconds\":2},\"volumeMounts\":[{\"mountPath\":\"/keys/github\",\"name\":\"github-app-private-key\",\"readOnly\":true}]}],\"restartPolicy\":\"Always\",\"securityContext\":{\"runAsUser\":0},\"serviceAccount\":\"kotsadm-api\",\"volumes\":[{\"name\":\"github-app-private-key\",\"secret\":{\"secretName\":\"github-app-private-key\"}}]}}}}\n"
	    }
	  },
	  "spec": {
	    "replicas": 1,
	    "selector": {
	      "matchLabels": {
		"app": "kotsadm-api"
	      }
	    },
	    "template": {
	      "metadata": {
		"creationTimestamp": null,
		"labels": {
		  "app": "kotsadm-api"
		}
	      },
	      "spec": {
		"containers": [
		  {
		    "name": "kotsadm-api",
		    "image": "localhost:32000/kotsadm-api:v1.0.1-30-g8fa13e34-dirty@sha256:4a0ca1a2eae46472bd2d454f9e763a458ddd172689e179b17054262507bb4fc8",
		    "ports": [
		      {
			"name": "http",
			"containerPort": 3000,
			"protocol": "TCP"
		      }
		    ],
		    "env": [
		      {
			"name": "DEV_NAMESPACE",
			"value": "test"
		      },
		      {
			"name": "POD_NAMESPACE",
			"valueFrom": {
			  "fieldRef": {
			    "apiVersion": "v1",
			    "fieldPath": "metadata.namespace"
			  }
			}
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
		"serviceAccountName": "kotsadm-api",
		"serviceAccount": "kotsadm-api",
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
		"lastUpdateTime": "2019-11-07T00:35:23Z",
		"lastTransitionTime": "2019-11-07T00:35:23Z",
		"reason": "MinimumReplicasAvailable",
		"message": "Deployment has minimum availability."
	      },
	      {
		"type": "Progressing",
		"status": "True",
		"lastUpdateTime": "2019-11-07T00:35:23Z",
		"lastTransitionTime": "2019-11-07T00:34:32Z",
		"reason": "NewReplicaSetAvailable",
		"message": "ReplicaSet \"kotsadm-api-6f4b994bd5\" has successfully progressed."
	      }
	    ]
	  }
	},
	{
	  "metadata": {
	    "name": "kotsadm-operator",
	    "namespace": "default",
	    "selfLink": "/apis/apps/v1/namespaces/default/deployments/kotsadm-operator",
	    "uid": "cfae9877-eef4-44c9-acac-0bf0d1aa547e",
	    "resourceVersion": "1583379",
	    "generation": 2,
	    "creationTimestamp": "2019-11-07T00:34:32Z",
	    "labels": {
	      "app.kubernetes.io/managed-by": "skaffold-v0.41.0"
	    },
	    "annotations": {
	      "deployment.kubernetes.io/revision": "2",
	      "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"kotsadm-operator\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"kotsadm-operator\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"kotsadm-operator\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"KOTSADM_API_ENDPOINT\",\"value\":\"http://kotsadm-api:3000\"},{\"name\":\"KOTSADM_TOKEN\",\"value\":\"***HIDDEN***\"},{\"name\":\"KOTSADM_TARGET_NAMESPACE\",\"value\":\"test\"}],\"image\":\"localhost:32000/kotsadm-operator:v1.0.1-30-g8fa13e34-dirty@sha256:177c15b6399717048e9355bc8fd8b8ed213be90615c7c6ee7b7fdcee50aca6c5\",\"imagePullPolicy\":\"Always\",\"name\":\"kotsadm-operator\",\"resources\":{\"limits\":{\"cpu\":\"200m\",\"memory\":\"1000Mi\"},\"requests\":{\"cpu\":\"100m\",\"memory\":\"500Mi\"}}}],\"restartPolicy\":\"Always\"}}}}\n"
	    }
	  },
	  "spec": {
	    "replicas": 1,
	    "selector": {
	      "matchLabels": {
		"app": "kotsadm-operator"
	      }
	    },
	    "template": {
	      "metadata": {
		"creationTimestamp": null,
		"labels": {
		  "app": "kotsadm-operator"
		}
	      },
	      "spec": {
		"containers": [
		  {
		    "name": "kotsadm-operator",
		    "image": "localhost:32000/kotsadm-operator:v1.0.1-30-g8fa13e34-dirty@sha256:177c15b6399717048e9355bc8fd8b8ed213be90615c7c6ee7b7fdcee50aca6c5",
		    "env": [
		      {
			"name": "KOTSADM_API_ENDPOINT",
			"value": "http://kotsadm-api:3000"
		      },
		      {
			"name": "KOTSADM_TOKEN",
			"value": "***HIDDEN***"
		      },
		      {
			"name": "KOTSADM_TARGET_NAMESPACE",
			"value": "test"
		      }
		    ],
		    "resources": {
		      "limits": {
			"cpu": "200m",
			"memory": "1000Mi"
		      },
		      "requests": {
			"cpu": "100m",
			"memory": "500Mi"
		      }
		    },
		    "terminationMessagePath": "/dev/termination-log",
		    "terminationMessagePolicy": "File",
		    "imagePullPolicy": "Always"
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
		"maxUnavailable": "25%",
		"maxSurge": "25%"
	      }
	    },
	    "revisionHistoryLimit": 10,
	    "progressDeadlineSeconds": 600
	  },
	  "status": {
	    "observedGeneration": 2,
	    "replicas": 1,
	    "updatedReplicas": 1,
	    "readyReplicas": 1,
	    "availableReplicas": 1,
	    "conditions": [
	      {
		"type": "Available",
		"status": "True",
		"lastUpdateTime": "2019-11-07T00:34:37Z",
		"lastTransitionTime": "2019-11-07T00:34:37Z",
		"reason": "MinimumReplicasAvailable",
		"message": "Deployment has minimum availability."
	      },
	      {
		"type": "Progressing",
		"status": "True",
		"lastUpdateTime": "2019-11-07T00:36:34Z",
		"lastTransitionTime": "2019-11-07T00:34:32Z",
		"reason": "NewReplicaSetAvailable",
		"message": "ReplicaSet \"kotsadm-operator-5b5c977699\" has successfully progressed."
	      }
	    ]
	  }
	},
	{
	  "metadata": {
	    "name": "kotsadm-postgres-watch",
	    "namespace": "default",
	    "selfLink": "/apis/apps/v1/namespaces/default/deployments/kotsadm-postgres-watch",
	    "uid": "ec195b07-4fd2-4bbe-8b01-4cbdfbe0e79d",
	    "resourceVersion": "1582762",
	    "generation": 1,
	    "creationTimestamp": "2019-11-07T00:34:39Z",
	    "annotations": {
	      "deployment.kubernetes.io/revision": "1"
	    },
	    "ownerReferences": [
	      {
		"apiVersion": "databases.schemahero.io/v1alpha2",
		"kind": "Database",
		"name": "kotsadm-postgres",
		"uid": "e6d51ce6-c4c1-428f-9e8e-56f1a3a43588",
		"controller": true,
		"blockOwnerDeletion": true
	      }
	    ]
	  },
	  "spec": {
	    "replicas": 1,
	    "selector": {
	      "matchLabels": {
		"deployment": "kotsadm-postgreswatch"
	      }
	    },
	    "template": {
	      "metadata": {
		"creationTimestamp": null,
		"labels": {
		  "deployment": "kotsadm-postgreswatch"
		}
	      },
	      "spec": {
		"containers": [
		  {
		    "name": "schemahero",
		    "image": "schemahero/schemahero:alpha",
		    "args": [
		      "watch",
		      "--driver",
		      "postgres",
		      "--uri",
		      "postgres://shipcloud:password@postgres.default.svc.cluster.local:5432/shipcloud?sslmode=disable",
		      "--namespace",
		      "default",
		      "--instance",
		      "kotsadm-postgres"
		    ],
		    "resources": {},
		    "terminationMessagePath": "/dev/termination-log",
		    "terminationMessagePolicy": "File",
		    "imagePullPolicy": "Always"
		  }
		],
		"restartPolicy": "Always",
		"terminationGracePeriodSeconds": 30,
		"dnsPolicy": "ClusterFirst",
		"serviceAccountName": "kotsadm-postgres",
		"serviceAccount": "kotsadm-postgres",
		"securityContext": {},
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
		"lastUpdateTime": "2019-11-07T00:35:00Z",
		"lastTransitionTime": "2019-11-07T00:35:00Z",
		"reason": "MinimumReplicasAvailable",
		"message": "Deployment has minimum availability."
	      },
	      {
		"type": "Progressing",
		"status": "True",
		"lastUpdateTime": "2019-11-07T00:35:00Z",
		"lastTransitionTime": "2019-11-07T00:34:39Z",
		"reason": "NewReplicaSetAvailable",
		"message": "ReplicaSet \"kotsadm-postgres-watch-5cf76f4c45\" has successfully progressed."
	      }
	    ]
	  }
	},
	{
	  "metadata": {
	    "name": "kotsadm-web",
	    "namespace": "default",
	    "selfLink": "/apis/apps/v1/namespaces/default/deployments/kotsadm-web",
	    "uid": "4e657f8a-5edb-498b-9402-1f93f51f5dda",
	    "resourceVersion": "1582354",
	    "generation": 1,
	    "creationTimestamp": "2019-11-07T00:34:32Z",
	    "labels": {
	      "app.kubernetes.io/managed-by": "skaffold-v0.41.0",
	      "skaffold.dev/builder": "local",
	      "skaffold.dev/cleanup": "true",
	      "skaffold.dev/deployer": "kustomize",
	      "skaffold.dev/docker-api-version": "1.40",
	      "skaffold.dev/run-id": "98f0a02b-9739-4d94-ba11-3e4d273c743e",
	      "skaffold.dev/tag-policy": "git-commit",
	      "skaffold.dev/tail": "true"
	    },
	    "annotations": {
	      "deployment.kubernetes.io/revision": "1",
	      "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"},\"name\":\"kotsadm-web\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"kotsadm-web\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"kotsadm-web\",\"app.kubernetes.io/managed-by\":\"skaffold-v0.41.0\",\"skaffold.dev/builder\":\"local\",\"skaffold.dev/cleanup\":\"true\",\"skaffold.dev/deployer\":\"kustomize\",\"skaffold.dev/docker-api-version\":\"1.40\",\"skaffold.dev/run-id\":\"98f0a02b-9739-4d94-ba11-3e4d273c743e\",\"skaffold.dev/tag-policy\":\"git-commit\",\"skaffold.dev/tail\":\"true\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"GITHUB_CLIENT_ID\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"client-id\",\"name\":\"github-app\"}}},{\"name\":\"GITHUB_INSTALL_URL\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"install-url\",\"name\":\"github-app\"}}},{\"name\":\"SHIP_CLUSTER_API_SERVER\",\"value\":\"http://localhost:30065\"},{\"name\":\"SHIP_CLUSTER_WEB_URI\",\"value\":\"http://localhost:8000\"}],\"image\":\"localhost:32000/kotsadm-web:v1.0.1-30-g8fa13e34@sha256:5b5b5b640b6e09d8b3185d4ae15ac4dc558d4e2ea034ac3e567d8cce04eadb9c\",\"imagePullPolicy\":\"IfNotPresent\",\"name\":\"kotsadm-web\",\"ports\":[{\"containerPort\":8000,\"name\":\"http\"}]}]}}}}\n"
	    }
	  },
	  "spec": {
	    "replicas": 1,
	    "selector": {
	      "matchLabels": {
		"app": "kotsadm-web"
	      }
	    },
	    "template": {
	      "metadata": {
		"creationTimestamp": null,
		"labels": {
		  "app": "kotsadm-web",
		  "app.kubernetes.io/managed-by": "skaffold-v0.41.0",
		  "skaffold.dev/builder": "local",
		  "skaffold.dev/cleanup": "true",
		  "skaffold.dev/deployer": "kustomize",
		  "skaffold.dev/docker-api-version": "1.40",
		  "skaffold.dev/run-id": "98f0a02b-9739-4d94-ba11-3e4d273c743e",
		  "skaffold.dev/tag-policy": "git-commit",
		  "skaffold.dev/tail": "true"
		}
	      },
	      "spec": {
		"containers": [
		  {
		    "name": "kotsadm-web",
		    "image": "localhost:32000/kotsadm-web:v1.0.1-30-g8fa13e34@sha256:5b5b5b640b6e09d8b3185d4ae15ac4dc558d4e2ea034ac3e567d8cce04eadb9c",
		    "ports": [
		      {
			"name": "http",
			"containerPort": 8000,
			"protocol": "TCP"
		      }
		    ],
		    "env": [
		      {
			"name": "GITHUB_CLIENT_ID",
			"valueFrom": {
			  "secretKeyRef": {
			    "name": "github-app",
			    "key": "client-id"
			  }
			}
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
		"lastUpdateTime": "2019-11-07T00:34:39Z",
		"lastTransitionTime": "2019-11-07T00:34:39Z",
		"reason": "MinimumReplicasAvailable",
		"message": "Deployment has minimum availability."
	      },
	      {
		"type": "Progressing",
		"status": "True",
		"lastUpdateTime": "2019-11-07T00:34:39Z",
		"lastTransitionTime": "2019-11-07T00:34:32Z",
		"reason": "NewReplicaSetAvailable",
		"message": "ReplicaSet \"kotsadm-web-79bfb95c48\" has successfully progressed."
	      }
	    ]
	  }
	}
      ]`

var collectedNodes = `[
	{
		"apiVersion": "v1",
		"kind": "Node",
		"metadata": {
		    "annotations": {
			"node.alpha.kubernetes.io/ttl": "0",
			"volumes.kubernetes.io/controller-managed-attach-detach": "true"
		    },
		    "creationTimestamp": "2019-10-23T18:16:43Z",
		    "labels": {
			"beta.kubernetes.io/arch": "amd64",
			"beta.kubernetes.io/os": "linux",
			"kubernetes.io/arch": "amd64",
			"kubernetes.io/hostname": "repldev-marc",
			"kubernetes.io/os": "linux",
			"microk8s.io/cluster": "true"
		    },
		    "name": "repldev-marc",
		    "resourceVersion": "1769699",
		    "selfLink": "/api/v1/nodes/repldev-marc",
		    "uid": "cd30c57f-b445-437f-9473-f13343124030"
		},
		"spec": {},
		"status": {
		    "addresses": [
			{
			    "address": "10.168.0.26",
			    "type": "InternalIP"
			},
			{
			    "address": "repldev-marc",
			    "type": "Hostname"
			}
		    ],
		    "allocatable": {
			"cpu": "8",
			"ephemeral-storage": "1015018628Ki",
			"hugepages-1Gi": "0",
			"hugepages-2Mi": "0",
			"memory": "30770604Ki",
			"pods": "110"
		    },
		    "capacity": {
			"cpu": "8",
			"ephemeral-storage": "1016067204Ki",
			"hugepages-1Gi": "0",
			"hugepages-2Mi": "0",
			"memory": "30873004Ki",
			"pods": "110"
		    },
		    "conditions": [
			{
			    "lastHeartbeatTime": "2019-11-08T17:03:39Z",
			    "lastTransitionTime": "2019-10-31T21:28:36Z",
			    "message": "kubelet has sufficient memory available",
			    "reason": "KubeletHasSufficientMemory",
			    "status": "False",
			    "type": "MemoryPressure"
			},
			{
			    "lastHeartbeatTime": "2019-11-08T17:03:39Z",
			    "lastTransitionTime": "2019-10-31T21:28:36Z",
			    "message": "kubelet has no disk pressure",
			    "reason": "KubeletHasNoDiskPressure",
			    "status": "False",
			    "type": "DiskPressure"
			},
			{
			    "lastHeartbeatTime": "2019-11-08T17:03:39Z",
			    "lastTransitionTime": "2019-10-31T21:28:36Z",
			    "message": "kubelet has sufficient PID available",
			    "reason": "KubeletHasSufficientPID",
			    "status": "False",
			    "type": "PIDPressure"
			},
			{
			    "lastHeartbeatTime": "2019-11-08T17:03:39Z",
			    "lastTransitionTime": "2019-10-31T21:28:36Z",
			    "message": "kubelet is posting ready status. AppArmor enabled",
			    "reason": "KubeletReady",
			    "status": "True",
			    "type": "Ready"
			}
		    ],
		    "daemonEndpoints": {
			"kubeletEndpoint": {
			    "Port": 10250
			}
		    },
		    "images": [
			{
			    "names": [
				"localhost:32000/kotsadm-api@sha256:d4821b65869454dfac53ad01f295740df6fcd52711f0dcf6aa9d7e515f7ebe3c"
			    ],
			    "sizeBytes": 755312372
			},
			{
			    "names": [
				"localhost:32000/kotsadm-api@sha256:fc3c971facc9dbd1b07e19c1ebb33c6361dd219af8efed0616afd1278f81fa4e"
			    ],
			    "sizeBytes": 755312032
			}
		    ],
		    "nodeInfo": {
			"architecture": "amd64",
			"bootID": "3401cdf2-129c-473d-a50c-723afd7378d3",
			"containerRuntimeVersion": "containerd://1.2.5",
			"kernelVersion": "5.0.0-1021-gcp",
			"kubeProxyVersion": "v1.16.2",
			"kubeletVersion": "v1.16.2",
			"machineID": "97f4a34d2aa9e26785177a6b64fb9108",
			"operatingSystem": "linux",
			"osImage": "Ubuntu 18.04.2 LTS",
			"systemUUID": "9dc594e5-ac7b-c649-e61f-cad715a28f79"
		    }
		}
	    }
]`
