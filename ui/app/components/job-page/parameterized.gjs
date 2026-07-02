/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import JobPage from 'nomad-ui/components/job-page';

export const JobPageParameterized = <template>
  <JobPage @job={{@job}} as |jobPage|>
    <jobPage.ui.Body>
      <jobPage.ui.Error />
      <jobPage.ui.Title>
        <span class="tag is-hollow">
          Parameterized
        </span>
      </jobPage.ui.Title>
      <jobPage.ui.StatsBox />
      <jobPage.ui.Summary />
      <jobPage.ui.Children
        @sortProperty={{@sortProperty}}
        @sortDescending={{@sortDescending}}
        @currentPage={{@currentPage}}
        @jobs={{@childJobs}}
      />
      <jobPage.ui.Meta />
    </jobPage.ui.Body>
  </JobPage>
</template>;

export default JobPageParameterized;
