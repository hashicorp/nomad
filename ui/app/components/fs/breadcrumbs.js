/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { isEmpty } from '@ember/utils';
import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('nav')
@classNames('breadcrumb')
@attributeBindings('data-test-fs-breadcrumbs')
export default class Breadcrumbs extends Component {
  'data-test-fs-breadcrumbs' = true;

  allocation = null;
  taskState = null;
  path = null;

  @computed('path')
  get breadcrumbs() {
    const breadcrumbs = this.path
      .split('/')
      .reject(isEmpty)
      .reduce((breadcrumbs, pathSegment, index) => {
        let breadcrumbPath;

        if (index > 0) {
          const lastBreadcrumb = breadcrumbs[index - 1];
          breadcrumbPath = `${lastBreadcrumb.path}/${pathSegment}`;
        } else {
          breadcrumbPath = pathSegment;
        }

        breadcrumbs.push({
          name: pathSegment,
          path: breadcrumbPath,
        });

        return breadcrumbs;
      }, []);

    if (breadcrumbs.length) {
      breadcrumbs[breadcrumbs.length - 1].isLast = true;
    }

    return breadcrumbs;
  }
}
