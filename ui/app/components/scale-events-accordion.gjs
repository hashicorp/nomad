/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat } from '@ember/helper';
import {
  HdsIcon,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import formatTs from 'nomad-ui/helpers/format-ts';
import JsonViewer from 'nomad-ui/components/json-viewer';
import ListAccordion from 'nomad-ui/components/list-accordion';

export const ScaleEventsAccordion = <template>
  <ListAccordion data-test-scale-events @source={{@events}} @key="time" as |a|>
    <a.head
      @buttonLabel="details"
      @isExpandable={{a.item.hasMeta}}
      class="with-columns"
    >
      <div class="columns inline-definitions">
        <div class="column is-3">
          <span class="icon-field">
            <span
              class="icon-container"
              title="{{if a.item.error 'Error event'}}"
              data-test-error={{a.item.error}}
            >
              {{#if a.item.error}}
                <HdsIcon
                  @name="x-circle-fill"
                  @isInline={{true}}
                  @color="critical"
                />
              {{/if}}
            </span>
            <span data-test-time title="{{formatTs a.item.time}}">
              {{formatMonthTs a.item.time}}
            </span>
          </span>
        </div>
        <div class="column is-2">
          {{#if a.item.hasCount}}
            <HdsTooltipButton
              data-test-count-icon
              @text={{concat
                "Count "
                (if a.item.increased "increased" "decreased")
                " to "
                a.item.count
              }}
              aria-label="More information"
            >
              {{#if a.item.increased}}
                <HdsIcon @name="arrow-up" @isInline={{true}} />
              {{else}}
                <HdsIcon @name="arrow-down" @isInline={{true}} />
              {{/if}}
            </HdsTooltipButton>
            <span data-test-count>
              {{a.item.count}}
            </span>
          {{/if}}
        </div>
        <div class="column" data-test-message>
          {{a.item.message}}
        </div>
      </div>
    </a.head>
    <a.body @fullBleed={{true}}>
      <JsonViewer @json={{a.item.meta}} @fluidHeight={{true}} />
    </a.body>
  </ListAccordion>
</template>;

export default ScaleEventsAccordion;
