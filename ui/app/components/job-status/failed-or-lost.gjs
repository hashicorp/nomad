/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, hash } from '@ember/helper';
import {
  HdsIcon,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';
import ConditionalLinkTo from 'nomad-ui/components/conditional-link-to';

export const JobStatusFailedOrLost = <template>
  <section class="failed-or-lost">
    <h4>Replaced Allocations</h4>
    <div class="failed-or-lost-links">
      {{#if @supportsRescheduling}}
        <span>
          <HdsTooltipButton
            @text="Allocations that have been rescheduled, on another node if possible, due to failure or manual restart"
            aria-label="Info"
          >
            <HdsIcon @name="info" @isInline={{true}} />
          </HdsTooltipButton>
          <ConditionalLinkTo
            @condition={{@rescheduledAllocs.length}}
            @route="jobs.job.allocations"
            @model={{@job}}
            @query={{hash
              scheduling='["has-been-rescheduled"]'
              version=(concat "[" @job.latestDeployment.versionNumber "]")
            }}
            @label="View Allocations"
          >
            {{@rescheduledAllocs.length}}
            Rescheduled
          </ConditionalLinkTo>
        </span>
      {{/if}}

      <span>
        <HdsTooltipButton
          @text="Allocations that have been restarted in-place due to a task failure or manual restart"
          aria-label="Info"
        >
          <HdsIcon @name="info" @isInline={{true}} />
        </HdsTooltipButton>
        <ConditionalLinkTo
          @condition={{@restartedAllocs.length}}
          @route="jobs.job.allocations"
          @model={{@job}}
          @query={{hash
            scheduling='["has-been-restarted"]'
            version=(concat "[" @job.latestDeployment.versionNumber "]")
          }}
          @label="View Allocations"
        >
          {{@restartedAllocs.length}}
          Restarted
        </ConditionalLinkTo>
      </span>
    </div>
  </section>
</template>;

export default JobStatusFailedOrLost;
