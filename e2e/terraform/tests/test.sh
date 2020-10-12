#!/bin/bash
# test the profiles by running Terraform plans and extracting the
# plans into JSON for comparison
set -eu

command -v jq > /dev/null || (echo "jq required"; exit 1 )
command -v terraform > /dev/null || (echo "terraform required"; exit 1 )

tempdir=$(mktemp -d)

plan() {
    vars_file="$1"
    out_file="$2"
    terraform plan --var-file="$vars_file" -out="$out_file" > /dev/null
}

# read the plan file to extract the bits we care about into JSON, and
# then compare this to the expected file.
check() {
    plan_file="$1"
    expected_file="$2"

    got=$(terraform show -json "$plan_file" \
              | jq --sort-keys --raw-output '
([.resource_changes[]
    | select(.name == "provision_nomad")
    | select(.change.actions[0] == "create")]
    | reduce .[] as $i ({};
         .[($i.module_address|ltrimstr("module."))] =
         .[($i.module_address|ltrimstr("module."))]
         + $i.change.after.triggers.script)
) as $provisioning |
{
    provisioning: $provisioning
}
')

    # leaves behind the temp plan file for debugging
    diff "$expected_file" <(echo "$got")

}

run() {
    echo -n "testing $1-test.tfvars... "
    plan "$1-test.tfvars" "${tempdir}/$1.plan"
    check "${tempdir}/$1.plan" "$1-expected.json"
    echo "ok!"
}

for t in *-test.tfvars; do
    run $(echo $t | grep -o '[0-9]\+')
done

rm -r "${tempdir}"
