package analyzer

var collectedNodes = `
[
  {
    "metadata": {
      "name": "ip-192-168-28-59.us-east-2.compute.internal",
      "selfLink": "/api/v1/nodes/ip-192-168-28-59.us-east-2.compute.internal",
      "uid": "6d679c35-1dfe-11ea-89c7-0ab299bbd38c",
      "resourceVersion": "7417849",
      "creationTimestamp": "2019-12-13T23:15:18Z",
      "labels": {
        "alpha.eksctl.io/cluster-name": "schemahero-demo",
        "alpha.eksctl.io/instance-id": "i-026467ed98dc19788",
        "alpha.eksctl.io/nodegroup-name": "ng-f2f5f9a5",
        "beta.kubernetes.io/arch": "amd64",
        "beta.kubernetes.io/instance-type": "m5.large",
        "beta.kubernetes.io/os": "linux",
        "failure-domain.beta.kubernetes.io/region": "us-east-2",
        "failure-domain.beta.kubernetes.io/zone": "us-east-2a",
        "kubernetes.io/arch": "amd64",
        "kubernetes.io/hostname": "ip-192-168-28-59.us-east-2.compute.internal",
        "kubernetes.io/os": "linux"
      },
      "annotations": {
        "node.alpha.kubernetes.io/ttl": "0",
        "volumes.kubernetes.io/controller-managed-attach-detach": "true"
      }
    },
    "spec": {
      "providerID": "aws:///us-east-2a/i-026467ed98dc19788"
    },
    "status": {
      "capacity": {
        "attachable-volumes-aws-ebs": "25",
        "cpu": "2",
        "ephemeral-storage": "20959212Ki",
        "hugepages-1Gi": "0",
        "hugepages-2Mi": "0",
        "memory": "7951376Ki",
        "pods": "29"
      },
      "allocatable": {
        "attachable-volumes-aws-ebs": "25",
        "cpu": "2",
        "ephemeral-storage": "19316009748",
        "hugepages-1Gi": "0",
        "hugepages-2Mi": "0",
        "memory": "7848976Ki",
        "pods": "29"
      },
      "conditions": [
        {
          "type": "MemoryPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:50:15Z",
          "lastTransitionTime": "2020-01-22T02:01:14Z",
          "reason": "KubeletHasSufficientMemory",
          "message": "kubelet has sufficient memory available"
        },
        {
          "type": "DiskPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:50:15Z",
          "lastTransitionTime": "2020-01-24T01:31:53Z",
          "reason": "KubeletHasNoDiskPressure",
          "message": "kubelet has no disk pressure"
        },
        {
          "type": "PIDPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:50:15Z",
          "lastTransitionTime": "2020-01-22T02:01:14Z",
          "reason": "KubeletHasSufficientPID",
          "message": "kubelet has sufficient PID available"
        },
        {
          "type": "Ready",
          "status": "True",
          "lastHeartbeatTime": "2020-01-29T18:50:15Z",
          "lastTransitionTime": "2020-01-22T02:01:14Z",
          "reason": "KubeletReady",
          "message": "kubelet is posting ready status"
        }
      ],
      "addresses": [
        {
          "type": "InternalIP",
          "address": "***HIDDEN***"
        },
        {
          "type": "ExternalIP",
          "address": "***HIDDEN***"
        },
        {
          "type": "Hostname",
          "address": "ip-192-168-28-59.us-east-2.compute.internal"
        },
        {
          "type": "InternalDNS",
          "address": "ip-192-168-28-59.us-east-2.compute.internal"
        },
        {
          "type": "ExternalDNS",
          "address": "ec2-3-133-126-65.us-east-2.compute.amazonaws.com"
        }
      ],
      "daemonEndpoints": {
        "kubeletEndpoint": {
          "Port": 10250
        }
      },
      "nodeInfo": {
        "machineID": "ec2d1877f782e9caa8d0f7cb5c6154b8",
        "systemUUID": "EC2D1877-F782-E9CA-A8D0-F7CB5C6154B8",
        "bootID": "8e91eddd-e115-4efe-a4e1-a32affdbab61",
        "kernelVersion": "4.14.146-119.123.amzn2.x86_64",
        "osImage": "Amazon Linux 2",
        "containerRuntimeVersion": "docker://18.6.1",
        "kubeletVersion": "v1.14.7-eks-1861c5",
        "kubeProxyVersion": "v1.14.7-eks-1861c5",
        "operatingSystem": "linux",
        "architecture": "amd64"
      },
      "images": [
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:257efb64c42c4e83f51618bc94b2898687292b7a1763c8c1165a0b8fb52b2c47",
            "kotsadm/kotsadm-api:v1.11.4"
          ],
          "sizeBytes": 1025604685
        },
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:bbdaf7b3abf9864953e3a25fb5d58746ee8b3056d4dbf1c9c477c4c08d7b3e6f",
            "kotsadm/kotsadm-api:v1.11.1"
          ],
          "sizeBytes": 1025603901
        },
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:0781a0d3ab73147db616a5359e7385dad5c1f942eb4dbf0032d965fc56342600"
          ],
          "sizeBytes": 1025603901
        },
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:29084f5f9896baaf947caf96b56e05af1d28a662dd437f222063df7d835f90e4"
          ],
          "sizeBytes": 1025603901
        },
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:000572c198f73af001713b10bd4869710c99313dde81d5589445a069271c0338"
          ],
          "sizeBytes": 1025603901
        },
        {
          "names": [
            "sentry@sha256:5a9fb82278c8ee4deb0fc9cb98dfcb6e1e0e184f7267a6a2c9074e0c687a0cd2",
            "sentry:9.1.2"
          ],
          "sizeBytes": 868746022
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/amazon-k8s-cni@sha256:c071dfc45cd957fc6ab2db769ae6374b1f59a08db90b0ff0b9166b8531497a35",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/amazon-k8s-cni:v1.5.3"
          ],
          "sizeBytes": 290731139
        },
        {
          "names": [
            "bitnami/postgresql@sha256:7b8f251a3ffdc3a5392b6b7bd1ac863d34f7cb1e9cc0ec3b2f92a45f9570eae5",
            "bitnami/postgresql:11.5.0-debian-9-r60"
          ],
          "sizeBytes": 165095931
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:1c19f3d507876e62889c0f592b20e15324effc579f2cd0591039fa0cdbac633d",
            "kotsadm/kotsadm-migrations:v1.11.1"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:5ae3fd834b72a37d92c801cc5b281b2339c17865be0c5298e4fdff62a9c4dde4"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:c1a7dce8fc27c2fedbf567370d24e0a3759c1840fa16a46a8569d6c4a3e09152",
            "kotsadm/kotsadm-migrations:v1.11.4"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "bitnami/redis@sha256:505188ab03eae7d63902fed9e2ab1bcfc2bf98a0244ba69f488cc6018eb6f330",
            "bitnami/redis:5.0.5-debian-9-r141"
          ],
          "sizeBytes": 96707700
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/kube-proxy@sha256:d3a6122f63202665aa50f3c08644ef504dbe56c76a1e0ab05f8e296328f3a6b4",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/kube-proxy:v1.14.6"
          ],
          "sizeBytes": 82044796
        },
        {
          "names": [
            "bitnami/minideb@sha256:7f79535202f3610cf637b4ce9d92d7e28600ce9d7e05284f7c861c6ef35dcd1f",
            "bitnami/minideb:stretch"
          ],
          "sizeBytes": 53743451
        },
        {
          "names": [
            "ttl.sh/sdfsdfsdf/minideb@sha256:b02b0c29f37f90a013e0a7a38f47667a219a5785b55aba6af0bbb54c5ad691b8",
            "ttl.sh/sdfsdfsdf/minideb:stretch"
          ],
          "sizeBytes": 53743418
        },
        {
          "names": [
            "kotsadm/minio@sha256:a68fb7b34d58c8167d11a93ebe887ab44ccb9447593e9ce7c36ac940c78221d4",
            "kotsadm/minio:v1.11.1"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/coredns@sha256:c85954b828a5627b9f3c4540893ab9d8a4be5f8da7513882ad122e08f5c2e60a",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/coredns:v1.3.1"
          ],
          "sizeBytes": 35174083
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/pause-amd64@sha256:bea77c323c47f7b573355516acf927691182d1333333d1f41b7544012fab7adf",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/pause-amd64:3.1"
          ],
          "sizeBytes": 742472
        }
      ]
    }
  },
  {
    "metadata": {
      "name": "ip-192-168-71-129.us-east-2.compute.internal",
      "selfLink": "/api/v1/nodes/ip-192-168-71-129.us-east-2.compute.internal",
      "uid": "6c8f3260-1dfe-11ea-89c7-0ab299bbd38c",
      "resourceVersion": "7417782",
      "creationTimestamp": "2019-12-13T23:15:16Z",
      "labels": {
        "alpha.eksctl.io/cluster-name": "schemahero-demo",
        "alpha.eksctl.io/instance-id": "i-0b7ad3f63b3a123b8",
        "alpha.eksctl.io/nodegroup-name": "ng-f2f5f9a5",
        "beta.kubernetes.io/arch": "amd64",
        "beta.kubernetes.io/instance-type": "m5.large",
        "beta.kubernetes.io/os": "linux",
        "failure-domain.beta.kubernetes.io/region": "us-east-2",
        "failure-domain.beta.kubernetes.io/zone": "us-east-2c",
        "kubernetes.io/arch": "amd64",
        "kubernetes.io/hostname": "ip-192-168-71-129.us-east-2.compute.internal",
        "kubernetes.io/os": "linux"
      },
      "annotations": {
        "node.alpha.kubernetes.io/ttl": "0",
        "volumes.kubernetes.io/controller-managed-attach-detach": "true"
      }
    },
    "spec": {
      "providerID": "aws:///us-east-2c/i-0b7ad3f63b3a123b8"
    },
    "status": {
      "capacity": {
        "attachable-volumes-aws-ebs": "25",
        "cpu": "2",
        "ephemeral-storage": "20959212Ki",
        "hugepages-1Gi": "0",
        "hugepages-2Mi": "0",
        "memory": "7865360Ki",
        "pods": "29"
      },
      "allocatable": {
        "attachable-volumes-aws-ebs": "25",
        "cpu": "2",
        "ephemeral-storage": "19316009748",
        "hugepages-1Gi": "0",
        "hugepages-2Mi": "0",
        "memory": "7762960Ki",
        "pods": "29"
      },
      "conditions": [
        {
          "type": "MemoryPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:49:36Z",
          "lastTransitionTime": "2019-12-13T23:15:16Z",
          "reason": "KubeletHasSufficientMemory",
          "message": "kubelet has sufficient memory available"
        },
        {
          "type": "DiskPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:49:36Z",
          "lastTransitionTime": "2020-01-22T14:30:11Z",
          "reason": "KubeletHasNoDiskPressure",
          "message": "kubelet has no disk pressure"
        },
        {
          "type": "PIDPressure",
          "status": "False",
          "lastHeartbeatTime": "2020-01-29T18:49:36Z",
          "lastTransitionTime": "2019-12-13T23:15:16Z",
          "reason": "KubeletHasSufficientPID",
          "message": "kubelet has sufficient PID available"
        },
        {
          "type": "Ready",
          "status": "True",
          "lastHeartbeatTime": "2020-01-29T18:49:36Z",
          "lastTransitionTime": "2019-12-13T23:16:06Z",
          "reason": "KubeletReady",
          "message": "kubelet is posting ready status"
        }
      ],
      "addresses": [
        {
          "type": "InternalIP",
          "address": "***HIDDEN***"
        },
        {
          "type": "ExternalIP",
          "address": "***HIDDEN***"
        },
        {
          "type": "Hostname",
          "address": "ip-192-168-71-129.us-east-2.compute.internal"
        },
        {
          "type": "InternalDNS",
          "address": "ip-192-168-71-129.us-east-2.compute.internal"
        },
        {
          "type": "ExternalDNS",
          "address": "ec2-3-18-214-18.us-east-2.compute.amazonaws.com"
        }
      ],
      "daemonEndpoints": {
        "kubeletEndpoint": {
          "Port": 10250
        }
      },
      "nodeInfo": {
        "machineID": "ec2502eb42ac572c0fc598fd2854029d",
        "systemUUID": "EC2502EB-42AC-572C-0FC5-98FD2854029D",
        "bootID": "d6ce6c46-98af-44c0-8f0a-7c6f0affba35",
        "kernelVersion": "4.14.146-119.123.amzn2.x86_64",
        "osImage": "Amazon Linux 2",
        "containerRuntimeVersion": "docker://18.6.1",
        "kubeletVersion": "v1.14.7-eks-1861c5",
        "kubeProxyVersion": "v1.14.7-eks-1861c5",
        "operatingSystem": "linux",
        "architecture": "amd64"
      },
      "images": [
        {
          "names": [
            "kotsadm/kotsadm-api@sha256:9bc79559156f04a1e086b865db962c7e3ca32575f654c0f09d5fdc4acf118d8a",
            "kotsadm/kotsadm-api:alpha"
          ],
          "sizeBytes": 1025599949
        },
        {
          "names": [
            "codescope/mjml-tcpserver@sha256:5a3f0c82a483f10255a06be5c74a34686f844b37b818a8b07c137f9c1bb1e8d7",
            "codescope/mjml-tcpserver:0.8.0"
          ],
          "sizeBytes": 920297781
        },
        {
          "names": [
            "sentry@sha256:5a9fb82278c8ee4deb0fc9cb98dfcb6e1e0e184f7267a6a2c9074e0c687a0cd2",
            "sentry:9.1.2"
          ],
          "sizeBytes": 868746022
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:849ba88648a4d85e8eff5c845477af593b139d0a695a1ad5c44ba0a9eec80b54"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:e8ebbbad7cdc44f9ddf8237ec38e75a9211d83e2ebec7dffd9c8c5f40f888cd3",
            "kotsadm/kotsadm-operator:v1.11.1"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:207b23336e6ac227c5bba39990107e42eaffe9f237e94e49abf9520e70826aa8",
            "kotsadm/kotsadm-operator:v1.11.0"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:9790dd5bc5450520db67b272b4bd38da595ee4d99af17307ae01ecf05b2844db"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:61fdc4c1b80106717ea364de6a1dff31f0634215d5408b4bca86e1cfa84f37eb"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:66cefcfd42ebab1ac441a9bf0ff755584d82631da853e40f84e5059ec928a985",
            "kotsadm/kotsadm-operator:v1.11.4"
          ],
          "sizeBytes": 478334600
        },
        {
          "names": [
            "kotsadm/kotsadm-operator@sha256:a19bf2afcbc318c169db4dbd6c6f8cdca02e6b0ee9922555441025f67c9e21f4",
            "kotsadm/kotsadm-operator:v1.10.3"
          ],
          "sizeBytes": 478317692
        },
        {
          "names": [
            "codescope/core@sha256:c8914b21b47d8394969e71054f1b964466fc1fe69cd70a778c142b947d8d08bf",
            "codescope/core:1.5.0"
          ],
          "sizeBytes": 405019853
        },
        {
          "names": [
            "kotsadm/kotsadm@sha256:6363777cbc9e57939ee33032dcfdd4619cee1b73428d031c8966948ec8172499",
            "kotsadm/kotsadm:v1.11.4"
          ],
          "sizeBytes": 300000312
        },
        {
          "names": [
            "kotsadm/kotsadm@sha256:dcff0ff224cb18e19026928a5d7a27ccfa9950032900b0c2a5fa5de8d6456ef2",
            "kotsadm/kotsadm:v1.11.1"
          ],
          "sizeBytes": 299996710
        },
        {
          "names": [
            "kotsadm/kotsadm@sha256:3fdeedc495df96c5831a3a198190c1b5b2708f5a438fea940f4798085e0a70c1"
          ],
          "sizeBytes": 299996710
        },
        {
          "names": [
            "kotsadm/kotsadm@sha256:271ba33be8a1d0d51fe387e7df8709809fcaa00a5501e7f107253afb5628999a"
          ],
          "sizeBytes": 299996560
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/amazon-k8s-cni@sha256:c071dfc45cd957fc6ab2db769ae6374b1f59a08db90b0ff0b9166b8531497a35",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/amazon-k8s-cni:v1.5.3"
          ],
          "sizeBytes": 290731139
        },
        {
          "names": [
            "kotsadm/kotsadm@sha256:08de237443b718d8b0ee260701dddc4b8c9b67fee5b5929b051a20312bf9aa39"
          ],
          "sizeBytes": 255742861
        },
        {
          "names": [
            "postgres@sha256:cc8fb6b149b387fed332b5bebd144f810df544e2df514383f82f6e61698b2aea",
            "postgres:10.7"
          ],
          "sizeBytes": 229651900
        },
        {
          "names": [
            "bitnami/postgresql@sha256:7b8f251a3ffdc3a5392b6b7bd1ac863d34f7cb1e9cc0ec3b2f92a45f9570eae5",
            "bitnami/postgresql:11.5.0-debian-9-r60"
          ],
          "sizeBytes": 165095931
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:9ebee83999219df4226d7f85b1da71420c3ebd3011cb79012a15c0fb805b9b3e"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:1c19f3d507876e62889c0f592b20e15324effc579f2cd0591039fa0cdbac633d"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:5ae3fd834b72a37d92c801cc5b281b2339c17865be0c5298e4fdff62a9c4dde4",
            "kotsadm/kotsadm-migrations:v1.11.1"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "kotsadm/kotsadm-migrations@sha256:1f467665d4e6714b19d8b82a9c859e958537c91452588c67a09db6f66b751af3",
            "kotsadm/kotsadm-migrations:alpha"
          ],
          "sizeBytes": 156079510
        },
        {
          "names": [
            "bitnami/redis@sha256:505188ab03eae7d63902fed9e2ab1bcfc2bf98a0244ba69f488cc6018eb6f330",
            "bitnami/redis:5.0.5-debian-9-r141"
          ],
          "sizeBytes": 96707700
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/kube-proxy@sha256:d3a6122f63202665aa50f3c08644ef504dbe56c76a1e0ab05f8e296328f3a6b4",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/kube-proxy:v1.14.6"
          ],
          "sizeBytes": 82044796
        },
        {
          "names": [
            "bitnami/minideb@sha256:7f79535202f3610cf637b4ce9d92d7e28600ce9d7e05284f7c861c6ef35dcd1f",
            "bitnami/minideb:stretch"
          ],
          "sizeBytes": 53743451
        },
        {
          "names": [
            "kotsadm/minio@sha256:3b1aadcd350f2c5b003b1e736bc89b23a636c2cf4eb3bbc7e459452a504e18ef",
            "kotsadm/minio:alpha"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "kotsadm/minio@sha256:a68fb7b34d58c8167d11a93ebe887ab44ccb9447593e9ce7c36ac940c78221d4",
            "kotsadm/minio:v1.11.1"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "kotsadm/minio@sha256:38c18cc2d92573cfce813931aaf04183b8c23b87a5ba0d672a8cfc1ca4f1acc6",
            "kotsadm/minio:v1.10.3"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "kotsadm/minio@sha256:1c7c8a0e953fccbe44f44134a29431fd86a5cfc7845b84adb12a808b503cf847",
            "kotsadm/minio:v1.11.0"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "kotsadm/minio@sha256:ffc3a26ce3fca3a6f5802444ceb6fee7a98a136c8e60b7f0020c6ce036ec628c",
            "kotsadm/minio:v1.11.4"
          ],
          "sizeBytes": 51885319
        },
        {
          "names": [
            "flungo/netutils@sha256:cf2a22cf9edee0640bae64fc33b8916fef524cc7f454e0279d91509cc1aecd60",
            "flungo/netutils:latest"
          ],
          "sizeBytes": 42668818
        },
        {
          "names": [
            "codescope/ui@sha256:147be2359690e9b4237462010111986234ec253cf19f2f7fed8e9a5c1ea59938",
            "codescope/ui:1.6.3"
          ],
          "sizeBytes": 25522501
        },
        {
          "names": [
            "codescope/router@sha256:fea5f5bf3b2fe8c872c769c91762108e7d6ff791e793c92d068035462d149de7",
            "codescope/router:0.4.2"
          ],
          "sizeBytes": 23233618
        },
        {
          "names": [
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/pause-amd64@sha256:bea77c323c47f7b573355516acf927691182d1333333d1f41b7544012fab7adf",
            "602401143452.dkr.ecr.us-east-2.amazonaws.com/eks/pause-amd64:3.1"
          ],
          "sizeBytes": 742472
        }
      ]
    }
  }
]

`

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
