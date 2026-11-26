# pyright: reportMissingImports=false

import time
import sys
import argparse
import json
import yaml

try:
    from kubernetes import config
    from kubernetes.client import V1DeleteOptions
    from kubernetes.client.api import core_v1_api
    from kubernetes.client.rest import ApiException
except ImportError as exc:
    raise ImportError(
        "The 'kubernetes' package is required. Install it with: pip install kubernetes"
    ) from exc


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("pod_name")
    parser.add_argument("-n", "--namespace", default="default")
    return parser.parse_args()


def get_containerid(core_v1, target_name, target_namespace):
    pod = core_v1.read_namespaced_pod(target_name, target_namespace)
    return pod.status.container_statuses[0].container_id


def get_node_name(core_v1, target_name, target_namespace):
    pod = core_v1.read_namespaced_pod(target_name, target_namespace)
    return pod.spec.node_name


def create_configmap(api_instance):
    config_txt = {
        "runtime-endpoint": "unix:///run/containerd/containerd.sock",
        "image-endpoint": "unix:///run/containerd/containerd.sock",
        "timeout": 2,
        "debug": False,
    }

    configmap_manifest = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {"name": "crictl-config"},
        "data": {
            "crictl.yaml": yaml.dump(config_txt),
        },
    }

    try:
        configmap = api_instance.create_namespaced_config_map(
            body=configmap_manifest, namespace="default"
        )
        return configmap.metadata.name

    except ApiException as e:
        if e.status == 409:
            print("This ConfigMap already exist.")
            return configmap_manifest["metadata"]["name"]
        raise


def run_commands(api_instance, container_id, node_name, conf_name):
    pod_manifest = {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {"generateName": "pod-analyst-"},
        "spec": {
            "hostPID": True,
            "nodeName": node_name,
            "restartPolicy": "Never",
            "containers": [
                {
                    "image": "busybox",
                    "name": "crictl",
                    "terminationMessagePath": "/dev/termination-log",
                    "terminationMessagePolicy": "FallbackToLogsOnError",
                    "volumeMounts": [
                        {
                            "mountPath": "/usr/bin/crictl",
                            "name": "crictl",
                        },
                        {
                            "mountPath": "/run/containerd/containerd.sock",
                            "name": "containerd-socket",
                        },
                        {
                            "mountPath": "/etc/crictl.yaml",
                            "subPath": "crictl.yaml",
                            "name": "config",
                            "readOnly": True,
                        },
                    ],
                    "args": [
                        "/usr/bin/crictl",
                        "inspect",
                        container_id,
                    ],
                }
            ],
            "volumes": [
                {
                    "name": "crictl",
                    "hostPath": {
                        "path": "/usr/bin/crictl",
                        "type": "File",
                    },
                },
                {
                    "name": "containerd-socket",
                    "hostPath": {
                        "path": "/run/containerd/containerd.sock",
                        "type": "Socket",
                    },
                },
                {
                    "name": "config",
                    "configMap": {
                        "name": conf_name,
                        "items": [{"key": "crictl.yaml", "path": "crictl.yaml"}],
                    },
                },
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
            rd = api_instance.read_namespaced_pod_log(
                name=name, namespace="default", _preload_content=False
            )
            logs = rd.data.decode("utf-8")
            logs = json.loads(logs)
            print(logs["info"]["pid"])
            sys.exit(0)
        elif resp.status.phase == "Failed":
            if resp.status.container_statuses[0].state is not None:
                if resp.status.container_statuses[0].state.terminated is not None:
                    print(
                        f"Reason: {resp.status.container_statuses[0].state.terminated.reason}\n"
                        f"Message: {resp.status.container_statuses[0].state.terminated.message}\n"
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
        api_instance.delete_namespaced_config_map(
            name=conf_name,
            namespace="default",
            grace_period_seconds=0,
            body=V1DeleteOptions(propagation_policy="Background"),
        )


def main():
    args = parse_args()
    target_name = args.pod_name
    target_namespace = args.namespace
    config.load_kube_config()
    core_v1 = core_v1_api.CoreV1Api()
    container_id = get_containerid(core_v1, target_name, target_namespace)
    container_id = container_id.split("://", 1)[-1]
    node_name = get_node_name(core_v1, target_name, target_namespace)
    conf_name = create_configmap(core_v1)
    run_commands(core_v1, container_id, node_name, conf_name)


if __name__ == "__main__":
    main()
