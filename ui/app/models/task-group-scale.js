/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { reads } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import {
  fragmentOwner,
  fragmentArray,
} from 'ember-data-model-fragments/attributes';

export default class TaskGroupScale extends Fragment {
  @fragmentOwner() jobScale;

  @attr('string') name;

  @attr('number') desired;
  @attr('number') placed;
  @attr('number') running;
  @attr('number') healthy;
  @attr('number') unhealthy;

  @fragmentArray('scale-event') events;

  @reads('events.length')
  isVisible;
}
