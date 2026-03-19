/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { HdsIcon } from '@hashicorp/design-system-components/components';

export const TaskContents = <template>
  <div class="border-and-label">
    <div class="border"></div>
    <div class="task-label">
      {{#if @active}}
        <HdsIcon
          @name="dot"
          @isInline={{true}}
          class="active-identifier icon-vertical-bump-down"
        />
      {{/if}}
      {{@task.name}}
    </div>
  </div>
  {{#if @shouldOpenInNewWindow}}
    <span class="tooltip" aria-label="Open in a new window">
      <HdsIcon
        @name="external-link"
        @color="faint"
        @isInline={{true}}
        class="show-on-hover"
      />
    </span>
  {{/if}}
</template>;

export default TaskContents;
