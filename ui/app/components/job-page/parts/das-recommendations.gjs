/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import can from 'ember-can/helpers/can';
import DasRecommendationAccordion from 'nomad-ui/components/das/recommendation-accordion';

export const JobPagePartsDasRecommendations = <template>
  {{#if (can "accept recommendations")}}
    {{#each @job.recommendationSummaries as |summary|}}
      <DasRecommendationAccordion @summary={{summary}} />
    {{/each}}
  {{/if}}
</template>;

export default JobPagePartsDasRecommendations;
