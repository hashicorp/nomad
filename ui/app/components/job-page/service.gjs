/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import JobPage from 'nomad-ui/components/job-page';

export const JobPageService = <template>
  <JobPage @job={{@job}} as |jobPage|>
    <jobPage.ui.Body>
      <jobPage.ui.Error />
      <jobPage.ui.Title />
      <jobPage.ui.StatsBox />
      <jobPage.ui.DasRecommendations />
      <jobPage.ui.PlacementFailures />
      <jobPage.ui.StatusPanel
        @statusMode={{@statusMode}}
        @setStatusMode={{@setStatusMode}}
      />
      <jobPage.ui.TaskGroups
        @sortProperty={{@sortProperty}}
        @sortDescending={{@sortDescending}}
      />
      <jobPage.ui.RecentAllocations
        @activeTask={{@activeTask}}
        @setActiveTaskQueryParam={{@setActiveTaskQueryParam}}
      />
      <jobPage.ui.Meta />
    </jobPage.ui.Body>
  </JobPage>
</template>;

export default JobPageService;
