/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import JobSubnav from 'nomad-ui/components/job-subnav';

export const JobPagePartsBody = <template>
  <JobSubnav @job={{@job}} />
  <section class="section">
    {{yield}}
  </section>
</template>;

export default JobPagePartsBody;
