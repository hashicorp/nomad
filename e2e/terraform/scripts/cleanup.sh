#!/usr/bin/env bash

set -e

LOCAL_NAME=$1


running_instances() {
    aws ec2 describe-instances \
        --filters "Name=key-name,Values=nomad-e2e-${LOCAL_NAME}" "Name=instance-state-name,Values=pending,running,shutting-down,stopping,stopped" \
        --query 'Reservations[*].Instances[*].InstanceId' \
        --output text
}

delete_instances() {
    instances=$(running_instances | tr '\n' ' ')
    if ! [[ "${instances}" ]]
    then
        echo "ec2: no running instances"
        return
    fi

    echo "ec2  running instances: ${instances}"
    aws ec2 terminate-instances --instance-ids ${instances} > /dev/null

    echo -n "  waiting for termination: "
    while [[ "$(running_instances)" ]]
    do
        echo -n .
        sleep 5
    done
    echo
}

efs_id() {
    aws efs describe-file-systems | jq -r ".FileSystems[] | select(.Name | contains(\"nomad-e2e-${LOCAL_NAME}\")) | .FileSystemId"
}

mount_target() {
    id=$(efs_id)
    if [[ "${id}" ]]
    then
        aws efs describe-mount-targets --file-system-id "${id}" | jq -r '.MountTargets[] | .MountTargetId'
    fi
}

efs_mount_count() {
    id=$1
    aws efs describe-file-systems --file-system-id "${id}" | jq '.FileSystems | map(.NumberOfMountTargets) | add'
}

delete_efs() {
    fs_id=$(efs_id)
    mount_id=$(mount_target)

    if [[ "${mount_id}" ]]
    then
        echo "fs mount target: ${mount_id}"
        aws efs delete-mount-target --mount-target-id "${mount_id}"
    fi

    if [[ "${fs_id}" ]]
    then
        echo "efs filesystem: ${fs_id}"

        echo -n "  waiting for deattachment: "
        while [[ "$(efs_mount_count $fs_id)" -ne 0 ]]
        do
            echo -n .
            sleep 1
        done
        echo .

        aws efs delete-file-system --file-system-id "${fs_id}"
    fi

}

ebs_volumes() {
    aws ec2 describe-volumes \
        --filters "Name=tag:Name,Values=nomad-e2e-${LOCAL_NAME}*" \
        --query 'Volumes[*].VolumeId' \
        --output text
}

delete_ebs_volumes() {
    volumes=$(ebs_volumes | tr '\n' ' ')
    if ! [[ "${volumes}" ]]
    then
        echo "ec2: no volumes"
        return 0
    fi

    echo "ec2 volumes: ${volumes}"
    echo "${volumes}" | xargs -n1 aws ec2 delete-volume --volume-id > /dev/null

    echo -n "  waiting for deletion: "
    while [[ "$(ebs_volumes)" ]]
    do
        echo -n .
        sleep 5
    done
    echo .
}

security_groups() {
    aws ec2 describe-security-groups \
        --filter "Name=group-name,Values=nomad-e2e-${LOCAL_NAME}*" \
        --query 'SecurityGroups[*].GroupId' \
        --output text
}

delete_sg_rules() {
    sg=${1}
    rules=$(aws ec2 describe-security-groups --group-id ${sg} --query "SecurityGroups[0].IpPermissions")
    if [[ "${rules}" != "[]" ]]; then
        aws ec2 revoke-security-group-ingress --cli-input-json "{\"GroupId\": \"${sg}\", \"IpPermissions\": ${rules}}" > /dev/null
    fi
}

delete_security_groups() {
    groups=$(security_groups | tr '\n' ' ')
    if ! [[ "${groups}" ]]
    then
        echo "ec2: no groups"
        return 0
    fi

    echo "ec2 groups: ${groups}"
    for sg in ${groups}; do
        delete_sg_rules "${sg}"
    done
    echo "${groups}" | xargs -n1 aws ec2 delete-security-group --group-id > /dev/null

    echo -n "  waiting for deletion: "
    while [[ "$(security_groups)" ]]
    do
        echo -n .
        sleep 5
    done
    echo .
}

delete_keypair() {
    aws ec2 delete-key-pair --key-name nomad-e2e-${LOCAL_NAME} > /dev/null
}

delete_instances
delete_efs
delete_ebs_volumes
delete_security_groups
delete_keypair
