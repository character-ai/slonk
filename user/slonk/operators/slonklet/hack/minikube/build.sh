eval $(minikube docker-env)
docker build -t test-slurmrestd:latest -f hack/test-slurmrestd/Dockerfile .
docker build -t slonklet-controller-image:latest -f hack/minikube/Dockerfile .
