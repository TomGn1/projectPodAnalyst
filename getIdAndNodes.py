import argparse
from kubernetes import client, config

def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("pod_name")
    parser.add_argument("-n", "--namespace", default="default")
    return parser.parse_args()

def get_pod_infos(v1, name, namespace):
    return v1.read_namespaced_pod(name, namespace)

def display(pod):
    print(
        f"POD: {pod.metadata.name} \nNODE: {pod.spec.node_name} \nCONTAINER: "
        )
    for cs in pod.status.container_statuses:
        print(
            f" - {cs.container_id}"
        )

def main():
	args = parse_args()
	name = args.pod_name
	namespace = args.namespace

	config.load_kube_config()
	v1 = client.CoreV1Api()

	pod = get_pod_infos(v1, name, namespace)
	display(pod)

if __name__ == "__main__":
	main()
