/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { HdsAlert } from '@hashicorp/design-system-components/components';
import PlacementFailure from 'nomad-ui/components/placement-failure';

const PlacementFailures = <template>
  {{#if @job.hasPlacementFailures}}
    <HdsAlert
      @type="inline"
      @color="critical"
      data-test-placement-failures
      class="boxed-section placement-failures"
      as |A|
    >
      <A.Title>Placement Failures</A.Title>
      <A.Description>
        {{#each @job.taskGroups as |taskGroup|}}
          <PlacementFailure @taskGroup={{taskGroup}} />
        {{/each}}
      </A.Description>
    </HdsAlert>
  {{/if}}
</template>;

export default PlacementFailures;
