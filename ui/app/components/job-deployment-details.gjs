/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { hash } from '@ember/helper';
import JobDeploymentDeploymentAllocations from 'nomad-ui/components/job-deployment/deployment-allocations';
import JobDeploymentDeploymentMetrics from 'nomad-ui/components/job-deployment/deployment-metrics';
import JobDeploymentTaskGroups from 'nomad-ui/components/job-deployment/task-groups';

export const JobDeploymentDetails = <template>
  {{yield
    (hash
      metrics=(component JobDeploymentDeploymentMetrics deployment=@deployment)
      taskGroups=(component JobDeploymentTaskGroups deployment=@deployment)
      allocations=(component
        JobDeploymentDeploymentAllocations deployment=@deployment
      )
    )
  }}
</template>;

export default JobDeploymentDetails;
