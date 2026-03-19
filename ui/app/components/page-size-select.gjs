/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import PowerSelect from 'ember-power-select/components/power-select';

export default class PageSizeSelect extends Component {
  @service userSettings;

  pageSizeOptions = [10, 25, 50];

  get onChange() {
    return this.args.onChange ?? (() => {});
  }

  handlePageSizeChange = (pageSize) => {
    this.userSettings.set('pageSize', pageSize);
    this.onChange(pageSize);
  };

  <template>
    <div
      class="field is-horizontal"
      data-test-page-size-select-parent
      ...attributes
    >
      <span class="field-label is-small">
        Per page
      </span>
      <PowerSelect
        @tagName="div"
        class="field-body"
        data-test-page-size-select
        @ariaLabel="label-page-size-select"
        @ariaLabelledBy="label-page-size-select"
        @options={{this.pageSizeOptions}}
        @selected={{this.userSettings.pageSize}}
        @onChange={{this.handlePageSizeChange}}
        as |option|
      >
        {{option}}
      </PowerSelect>
    </div>
  </template>
}
