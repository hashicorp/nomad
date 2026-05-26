/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import FsLink from 'nomad-ui/components/fs/link';

export default class Breadcrumbs extends Component {
  get breadcrumbs() {
    const path = this.args.path ?? '';
    const breadcrumbs = path
      .split('/')
      .filter(Boolean)
      .reduce((items, pathSegment, index) => {
        let breadcrumbPath;

        if (index > 0) {
          const lastBreadcrumb = items[index - 1];
          breadcrumbPath = `${lastBreadcrumb.path}/${pathSegment}`;
        } else {
          breadcrumbPath = pathSegment;
        }

        items.push({
          name: pathSegment,
          path: breadcrumbPath,
        });

        return items;
      }, []);

    if (breadcrumbs.length) {
      breadcrumbs[breadcrumbs.length - 1].isLast = true;
    }

    return breadcrumbs;
  }

  <template>
    <nav class="breadcrumb" data-test-fs-breadcrumbs>
      <ul>
        <li class={{if this.breadcrumbs "" "is-active"}}>
          <FsLink @allocation={{@allocation}} @taskState={{@taskState}}>
            {{if @taskState @taskState.name @allocation.shortId}}
          </FsLink>
        </li>
        {{#each this.breadcrumbs as |breadcrumb|}}
          <li class={{if breadcrumb.isLast "is-active"}}>
            <FsLink
              @allocation={{@allocation}}
              @taskState={{@taskState}}
              @path={{breadcrumb.path}}
            >
              {{breadcrumb.name}}
            </FsLink>
          </li>
        {{/each}}
      </ul>
    </nav>
  </template>
}
