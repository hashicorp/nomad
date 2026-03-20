/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { hash } from '@ember/helper';

export default class ActorsRelationships extends Component {
  @service actorsRelationships;

  <template>
    {{yield
      (hash fns=this.actorsRelationships.fns data=this.actorsRelationships.data)
    }}
  </template>
}
