/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `job "job-with-actions" {
  # Specifies the datacenter where this job should be run
  # This can be omitted and it will default to ["*"]
  datacenters = ["*"]

  # Run the job only on Linux or MacOS.
  constraint {
    attribute = "\${attr.kernel.name}"
    operator  = "set_contains_any"
    value     = "darwin,linux"
  }

  group "my_group" {
    # Specifies the number of instances of this group that should be running.
    # Use this to scale or parallelize your job.
    count = 3

    task "sleepy" {
      driver = "raw_exec"

      # The "command" stanza specifies the command to run.
      # This command will run a .sh file generated from the template stanza below,
      # which will sleep for 2 seconds and repeat until the job is stopped.
      config {
        command = "\${NOMAD_TASK_DIR}/sleepy.sh"
      }

      resources {
        memory = 16
        cpu = 16
      }

      template {
        data = <<EOH
#!/bin/bash
SLEEP_SECS=2
interruptable_sleep() { for i in $(seq 1 $((2*\${1}))); do sleep .5; done; }
sigint() { echo "$(date) - SIGTERM received; Ending."; exit 0; }
trap 'sigint'  INT
echo "$(date) - Starting. SLEEP_SECS=\${SLEEP_SECS}"
while true; do echo "$(date) - Sleeping for \${SLEEP_SECS} seconds."; interruptable_sleep \${SLEEP_SECS}; done
EOH
        destination = "local/sleepy.sh"
      }

      # Action blocks can be used to run arbitrary commands on an allocation,
      # such as to collect logs, clear caches, migrate databases, or debug issues.

      # This action will run a simple echo command every second.
      # Where the other actions run and exit upon completion, this action will run
      # until it is manually stopped with an escape character or socket closure.
      action "echo time" {
        command = "/bin/sh"
        args    = ["-c", "counter=0; while true; do echo \\"Running for \${counter} seconds\\"; counter=$((counter + 1)); sleep 1; done"]
      }
      
      # These next two actions use the nomad CLI to get information about the job and allocation.
      # The NOMAD_JOB_NAME and NOMAD_ALLOC_ID environment variables are made available to running
      # tasks and can be used in jobspecs.
      action "get-job-info" {
        command = "/bin/sh"
        args    = ["-c",
          <<EOT
          nomad job status \${NOMAD_JOB_NAME}
          EOT
        ]
      }

      action "get alloc info" {
        command = "/bin/sh"
        args    = ["-c",
          <<EOT
          nomad alloc status \${NOMAD_ALLOC_ID}
          EOT
        ]
      }

      # This final action will fetch the latest Nomad changelog and parse it to get the latest 3 versions
      # and the number of items under each section using curl and awk. This is meant to demonstrate
      # how actions can be used to fetch information or perform other tasks that are not directly related
      # to the running task.
      action "fetch-latest-nomad-changelog" {
        command = "/bin/sh"
        args    = ["-c", 
          <<EOT
          curl -s https://raw.githubusercontent.com/hashicorp/nomad/main/CHANGELOG.md | 
          awk 'BEGIN{
              RS="## "; FS="\\n"; 
              section=""; count=0
          }
          {
              if (count < 3 && NR > 1){
                  split($1, versionInfo, /[()]/);
                  version=versionInfo[1];
                  gsub(" ", "", version);
                  releaseDate=versionInfo[2];
                  urlVersion=version; gsub("\\\\.", "", urlVersion);
                  urlDate=releaseDate; gsub(" ", "-", urlDate); gsub(",", "", urlDate);
                  for(i=1; i<=NF; i++){
                      if($i ~ /^[A-Z ]+:$/){
                          gsub(":", "", $i);
                          section=$i;
                          itemCount[section]=0;
                      }
                      if(section && $i ~ /^\\*/){
                          itemCount[section]++;
                      }
                  }
                  printf "Version: %s\\nRelease Date: %s\\n", version, releaseDate;
                  for (s in itemCount) {
                      printf "%d %s, ", itemCount[s], s;
                  }
                  printf "\\nLink: https://github.com/hashicorp/nomad/blob/main/CHANGELOG.md#%s-%s\\n\\n", urlVersion, tolower(urlDate);
                  delete itemCount;
                  count++;
              }
          }'
          EOT
        ]
      }

    }
  }
}
`;
