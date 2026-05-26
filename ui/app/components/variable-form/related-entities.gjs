/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array } from '@ember/helper';
import { concat } from '@ember/helper';
import {
  HdsAlert,
  HdsLinkInline,
} from '@hashicorp/design-system-components/components';

export const VariableFormRelatedEntities = <template>
  <HdsAlert
    @type="inline"
    @color="highlight"
    @icon="info"
    class="related-entities notification"
    as |A|
  >
    <A.Title>Automatically-accessible variable</A.Title>
    <A.Description>
      This variable
      {{#if @new}}will be{{else}}is{{/if}}
      accessible by
      {{#if @task}}
        task
        <strong>{{@task}}</strong>
        in group
        <HdsLinkInline
          @route="jobs.job.task-group"
          @models={{array (concat @job "@" @namespace) @group}}
          @icon="external-link"
        >{{@group}}</HdsLinkInline>
      {{else if @group}}
        group
        <HdsLinkInline
          @route="jobs.job.task-group"
          @models={{array (concat @job "@" @namespace) @group}}
          @icon="external-link"
        >{{@group}}</HdsLinkInline>
      {{else if @job}}
        job
        <HdsLinkInline
          @route="jobs.job"
          @model={{concat @job "@" @namespace}}
          @icon="external-link"
        >{{@job}}</HdsLinkInline>
      {{else}}
        all nomad jobs in this namespace
      {{/if}}
    </A.Description>
  </HdsAlert>
</template>;

export default VariableFormRelatedEntities;
