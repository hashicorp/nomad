/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import classic from 'ember-classic-decorator';
import Fragment from 'ember-data-model-fragments/fragment';
import { get, computed } from '@ember/object';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import { fragment } from 'ember-data-model-fragments/attributes';

@classic
export default class NodeDriver extends Fragment {
  @fragmentOwner() node;

  @fragment('structured-attributes') attributes;

  @computed('name', 'attributes.structured')
  get attributesShort() {
    const attributes = this.get('attributes.structured');
    return get(attributes, `driver.${this.name}`);
  }

  @attr('string') name;
  @attr('boolean', { defaultValue: false }) detected;
  @attr('boolean', { defaultValue: false }) healthy;
  @attr('string') healthDescription;
  @attr('date') updateTime;

  @computed('healthy')
  get healthClass() {
    return this.healthy ? 'running' : 'failed';
  }
}
