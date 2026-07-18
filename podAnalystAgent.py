import time
import sys

from kubernetes import config
from kubernetes.client import V1DeleteOptions
from kubernetes.client.api import core_v1_api
from kubernetes.client.rest import ApiException


def run_commands(api_instance):
    pod_manifest = {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {"generateName": "pod-analyst-"},
        "spec": {
            "nodeName": "master",
            "restartPolicy": "Never",
            "containers": [
                {
                    "image": "busybox",
                    "name": "echo",
                    "args": ["/bin/sh", "-c", "echo hello"],
                }
            ],
        },
    }

    try:
        resp = api_instance.create_namespaced_pod(
            body=pod_manifest, namespace="default"
        )
        name = resp.metadata.name

    except ApiException as e:
        print(f"Erreur Kubernetes : {e}")
        return None

    start = time.time()
    resp = api_instance.read_namespaced_pod(name=name, namespace="default")
    final_states = ["Succeeded", "Failed"]
    timeout_seconds = 30

    try:
        while True:
            resp = api_instance.read_namespaced_pod(name=name, namespace="default")
            elapsed = time.time() - start
            if resp.status.phase in final_states or elapsed >= timeout_seconds:
                break
            time.sleep(1)

        if resp.status.phase == "Succeeded":
            rd = api_instance.read_namespaced_pod_log(name=name, namespace="default")
            print(rd)
            sys.exit(0)
        elif resp.status.phase == "Failed":
            if resp.status.container_statuses[0].state is not None:
                if resp.status.container_statuses[0].state.terminated is not None:
                    print(
                        f"Reason: {resp.status.container_statuses[0].state.terminated.reason}\n"
                        f"Exit code: {resp.status.container_statuses[0].state.terminated.exit_code}"
                    )

                    exit_code = resp.status.container_statuses[
                        0
                    ].state.terminated.exit_code
                    sys.exit(exit_code)

                else:
                    print("Container state unavailable")
            else:
                print("No container status available")

        elif elapsed >= timeout_seconds:
            if resp.status.phase == "Pending":
                if resp.status.container_statuses[0].state is not None:
                    if resp.status.container_statuses[0].state.waiting is not None:
                        print(
                            f"Reason: {resp.status.container_statuses[0].state.waiting.reason}\n"
                            f"Exit code: 124"
                        )
                else:
                    print(
                        f"Timeout after {timeout_seconds} seconds.\n"
                        f"Last observed phase: {resp.status.phase}."
                    )
            elif resp.status.phase == "Running":
                if resp.status.container_statuses[0].state is not None:
                    if resp.status.container_statuses[0].state.running is None:
                        print(
                            f"Reason: {resp.status.container_statuses[0].state.waiting.reason}\n"
                            f"Exit code: 124"
                        )
                else:
                    print(
                        f"Reason: Timeout after {timeout_seconds} seconds, pod phase is {resp.status.phase}.\nExit code: 124 "
                    )
            sys.exit(124)

    finally:
        api_instance.delete_namespaced_pod(
            name=name,
            namespace="default",
            grace_period_seconds=0,
            body=V1DeleteOptions(propagation_policy="Background"),
        )


def main():
    config.load_kube_config()
    core_v1 = core_v1_api.CoreV1Api()

    run_commands(core_v1)


if __name__ == "__main__":
    main()
