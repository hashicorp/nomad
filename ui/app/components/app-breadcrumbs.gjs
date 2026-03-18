/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import Breadcrumbs from 'nomad-ui/components/breadcrumbs';
import BreadcrumbsDefault from 'nomad-ui/components/breadcrumbs/default';
import BreadcrumbsJob from 'nomad-ui/components/breadcrumbs/job';

const isJobType = (type) => type === 'job';

export default class AppBreadcrumbsComponent extends Component {
  isOneCrumbUp = (iter = 0, totalNum = 0) => {
    return iter === totalNum - 2;
  };

  <template>
    <Breadcrumbs as |breadcrumbs|>
      {{#each breadcrumbs as |crumb iter|}}
        {{#let crumb.args.crumb as |c|}}
          {{#if (isJobType c.type)}}
            <BreadcrumbsJob
              @crumb={{c}}
              @isOneCrumbUp={{fn this.isOneCrumbUp iter breadcrumbs.length}}
            />
          {{else}}
            <BreadcrumbsDefault
              @crumb={{c}}
              @isOneCrumbUp={{fn this.isOneCrumbUp iter breadcrumbs.length}}
            />
          {{/if}}
        {{/let}}
      {{/each}}
    </Breadcrumbs>
  </template>
}
