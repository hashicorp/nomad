/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hash } from '@ember/helper';
import JobStatusPanel from 'nomad-ui/components/job-status/panel';
import JobPagePartsBody from 'nomad-ui/components/job-page/parts/body';
import JobPagePartsChildren from 'nomad-ui/components/job-page/parts/children';
import JobPagePartsDasRecommendations from 'nomad-ui/components/job-page/parts/das-recommendations';
import JobPagePartsError from 'nomad-ui/components/job-page/parts/error';
import JobPagePartsMeta from 'nomad-ui/components/job-page/parts/meta';
import JobPagePartsPlacementFailures from 'nomad-ui/components/job-page/parts/placement-failures';
import JobPagePartsRecentAllocations from 'nomad-ui/components/job-page/parts/recent-allocations';
import JobPagePartsStatsBox from 'nomad-ui/components/job-page/parts/stats-box';
import JobPagePartsSummary from 'nomad-ui/components/job-page/parts/summary';
import JobPagePartsTaskGroups from 'nomad-ui/components/job-page/parts/task-groups';
import JobPagePartsTitle from 'nomad-ui/components/job-page/parts/title';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

export default class JobPage extends Component {
  @tracked errorMessage = null;

  clearErrorMessage = () => {
    this.errorMessage = null;
  };

  handleError = (errorObject) => {
    this.errorMessage = errorObject;
  };

  setError = (err) => {
    this.errorMessage = {
      title: 'Could Not Force Launch',
      description: messageForError(err, 'submit jobs'),
    };
  };

  <template>
    {{yield
      (hash
        data=(hash)
        fns=(hash setError=this.setError)
        ui=(hash
          Body=(component JobPagePartsBody job=@job)
          Error=(component
            JobPagePartsError
            errorMessage=this.errorMessage
            onDismiss=this.clearErrorMessage
          )
          Title=(component
            JobPagePartsTitle job=@job handleError=this.handleError
          )
          StatsBox=(component JobPagePartsStatsBox job=@job)
          Summary=(component JobPagePartsSummary job=@job)
          PlacementFailures=(component JobPagePartsPlacementFailures job=@job)
          TaskGroups=(component JobPagePartsTaskGroups job=@job)
          RecentAllocations=(component
            JobPagePartsRecentAllocations
            job=@job
            activeTask=@activeTask
            setActiveTaskQueryParam=@setActiveTaskQueryParam
          )
          Meta=(component JobPagePartsMeta meta=@job.meta)
          DasRecommendations=(component JobPagePartsDasRecommendations job=@job)
          Children=(component JobPagePartsChildren job=@job)
          StatusPanel=(component
            JobStatusPanel job=@job handleError=this.handleError
          )
        )
      )
    }}
  </template>
}
