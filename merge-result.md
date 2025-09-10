Title: GPU and Accelerator Requirements
Requirement:
  - GPU Nodes: Minimum 4 nodes with NVIDIA GPUs
  - GPU Model: NVIDIA A100 or newer
  - CUDA Version: >= 12.0
  - GPU Memory per card: Minimum 40GB
  - NVIDIA GPU Operator: Required for automatic driver management
  - Device Plugin: nvidia-device-plugin-daemonset deployed

================================================================================

Title: Machine Learning Framework Requirements
Requirement:
  - Kubeflow Operator: Required
  - PyTorch Operator: Required
  - TensorFlow Operator: Required
  - MLflow tracking server: http://mlflow.ml-platform.svc.cluster.local:5000
  - Model registry: Harbor with OCI artifact support

================================================================================

Title: Storage Requirements for ML Workloads
Requirement:
  - Dataset Storage Class: fast-nvme
  - Model Storage Class: replicated-ssd
  - Shared filesystem: NFS or CSI driver with RWX
  - Minimum throughput: 500MB/s sequential read
  - IOPS requirement: 5000 for training data
  - S3-compatible object storage: Required at https://s3.ml-platform.local

================================================================================

Title: Network Requirements for Distributed Training
Requirement:
  - CNI Plugin: Cilium with RDMA support preferred
  - Network bandwidth: Minimum 100Gbps between nodes
  - InfiniBand: Required
  - Service mesh: Istio with telemetry disabled for training jobs
  - Load balancer: Required for model serving endpoints

================================================================================

Title: JupyterHub Platform Requirements
Requirement:
  - JupyterHub version: >= 4.0
  - User storage: 500Gi per user workspace
  - Default image: jupyter/tensorflow-notebook:cuda12
  - GPU passthrough: Enabled with scheduler hints
  - OAuth provider: OAuth2

================================================================================

Title: Node Pool Requirements for ML Workloads
Requirement:
  - CPU-only pool: Minimum 5 nodes for services
  - GPU pool: Minimum 4 nodes for training
  - Spot/Preemptible pool: 10 nodes for batch jobs
  - Node labels:
    - workload-type: cpu-compute | gpu-compute | spot-compute
    - node-lifecycle: on-demand | spot

================================================================================

Title: ML-Specific Monitoring Requirements
Requirement:
  - Metrics backend: Prometheus + VictoriaMetrics
  - GPU metrics: dcgm-exporter required
  - Training metrics: TensorBoard server deployment
  - Model metrics: Seldon Core Analytics
  - Experiment tracking: MLflow
  - Data drift detection: Optional

================================================================================

Title: ML Platform Security Requirements
Requirement:
  - Pod Security Standards: restricted enforced
  - Image scanning: Required with Trivy or Snyk
  - Secrets management: HashiCorp Vault or external KMS
  - Network policies: Required for namespace isolation
  - Audit logging: Enabled with model access tracking
  - Data encryption: At-rest and in-transit required

================================================================================

Title: Backup Requirements for ML Assets
Requirement:
  - Backup solution: Velero with Restic
  - Model snapshots: Daily backups with 90 day retention
  - Dataset backups: Incremental backups to object storage
  - Experiment tracking DB: Point-in-time recovery required
  - Backup storage: s3://ml-backups/production

================================================================================

Title: Autoscaling Requirements for ML Workloads
Requirement:
  - Cluster Autoscaler: Required with AWS provider
  - HPA: Required for inference services
  - VPA: Enabled for training jobs
  - KEDA: Required for event-driven scaling
  - Scale-down delay: 30m for GPU nodes
  - Max cluster size: 200 nodes


Title: Kubernetes Control Plane Requirements
Requirement:
  - Version:
    - Minimum: v1.27.0
    - Supported: v1.22.x – v1.29.x (stable releases only)
  - APIs required (must be enabled, GA):
    - admissionregistration.k8s.io/v1
    - apiextensions.k8s.io/v1
    - apps/v1
    - batch/v1
    - networking.k8s.io/v1
    - policy/v1
    - rbac.authorization.k8s.io/v1
    - storage.k8s.io/v1

================================================================================

Title: Container Runtime Requirements
Requirement:
  - Runtime: containerd (CRI) version ≥ 1.5
  - Kubelet cgroup driver: systemd
  - CRI socket path: /run/containerd/containerd.sock
  - Security hardening:
    - Seccomp: enabled (default profiles permitted)
    - AppArmor: enabled where supported

================================================================================

Title: Default StorageClass Requirements
Requirement:
  - A StorageClass named "fast-ssd" must exist and be annotated as cluster default
  - AccessMode: ReadWriteOnce (RWO) required (RWX optional)
  - VolumeBindingMode: WaitForFirstConsumer preferred
  - allowVolumeExpansion: true recommended
  - Baseline performance per volume:
    - Minimum: 5000 write IOPS, 10000 read IOPS
    - Recommended: 3000+ write IOPS, 6000+ read IOPS, 250+ MB/s throughput
  - Encryption at rest: enabled

================================================================================

Title: Cluster Size and Aggregate Capacity
Requirement:
  - Node count: Minimum 5 nodes (HA baseline), Recommended 7 nodes
  - Total CPU: Minimum 8 vCPU, Recommended 8+ vCPU
  - Total Memory: Minimum 32 GiB, Recommended 32+ GiB
  - Control plane sizing:
    - Managed control planes supported (EKS/GKE/AKS)
    - Self-managed: 3 control-plane nodes recommended

================================================================================

Title: Postgres Platform Requirements
Requirement:
  - Database: PostgreSQL 15+
  - Connection: postgresql://postgres@postgres-primary.database.svc.cluster.local:5432/production
  - StorageClass: fast-ssd with:
    - Latency p99 ≤ 5 ms
    - ≥ 3000 read IOPS, ≥ 1000 write IOPS
    - allowVolumeExpansion: true
  - Memory per node: Minimum 16 GiB; Recommended 32 GiB
  - CPU per node: Minimum 4 vCPU; Recommended 4+ vCPU

================================================================================

Title: Redis Platform Requirements
Requirement:
  - Database: Redis 7.2+
  - Connection: redis://default:@redis-sentinel.cache.svc.cluster.local:26379
  - Ephemeral storage per node: Minimum 40 GiB; Recommended 100 GiB
  - If persistence enabled: SSD-backed StorageClass with low-latency reads/writes
  - Memory per node: Baseline 8 GiB; Recommended sized to dataset with 30% headroom

================================================================================

Title: Required CRDs and Ingress Capabilities
Requirement:
  - Ingress Controller: Contour
  - CRD must be present:
    - Group: heptio.com
    - Kind: IngressRoute
    - Version: v1beta1 or later served version
  - Ingress capability:
    - Layer-7 HTTP/HTTPS routing with TLS termination supported
    - Wildcard certificates permitted (optional)
    - Custom domain: *.apps.production.example.com

================================================================================

Title: Monitoring and Observability Requirements
Requirement:
  - Monitoring: Prometheus
  - Metrics retention: 30 days
  - Storage required: 100Gi
  - Components:
    - Prometheus for metrics collection
    - Grafana for visualization
    - AlertManager for alerting

================================================================================

Title: OS and Kernel Requirements
Requirement:
  - Nodes: Linux x86_64 (amd64) or arm64 on supported distributions
  - Supported OS: Ubuntu 22.04 LTS, RHEL 9, Rocky Linux 9, Amazon Linux 2023,
  - Kernel: ≥ 5.15 with cgroups v1 or v2 (v2 preferred)
  - Time sync: chrony or systemd-timesyncd active; clock drift < 500 ms
  - Filesystems: ext4 or xfs for container layers and volumes
  - SELinux/AppArmor: enforcing/permissive accepted