/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array } from '@ember/helper';
import { LinkTo } from '@ember/routing';

export const FsLink = <template>
  {{#if @taskState}}
    {{#if @path}}
      <LinkTo
        @route="allocations.allocation.task.fs"
        @models={{array @allocation @taskState.name @path}}
        @activeClass="is-active"
      >
        {{yield}}
      </LinkTo>
    {{else}}
      <LinkTo
        @route="allocations.allocation.task.fs-root"
        @models={{array @allocation @taskState.name}}
        @activeClass="is-active"
      >
        {{yield}}
      </LinkTo>
    {{/if}}
  {{else}}
    {{#if @path}}
      <LinkTo
        @route="allocations.allocation.fs"
        @models={{array @allocation @path}}
        @activeClass="is-active"
      >
        {{yield}}
      </LinkTo>
    {{else}}
      <LinkTo
        @route="allocations.allocation.fs-root"
        @model={{@allocation}}
        @activeClass="is-active"
      >
        {{yield}}
      </LinkTo>
    {{/if}}
  {{/if}}
</template>;

export default FsLink;