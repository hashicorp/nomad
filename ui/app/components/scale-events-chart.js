/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { get } from '@ember/object';
import { copy } from 'ember-copy';

export default class ScaleEventsChart extends Component {
  /** Args
    events = []
  */

  @tracked activeEvent = null;

  get data() {
    const data = this.args.events.filterBy('hasCount').sortBy('time');

    // Extend the domain of the chart to the current time
    data.push({
      time: new Date(),
      count: data.lastObject.count,
    });

    // Make sure the domain of the chart includes the first annotation
    const firstAnnotation = this.annotations.sortBy('time')[0];
    if (firstAnnotation && firstAnnotation.time < data[0].time) {
      data.unshift({
        time: firstAnnotation.time,
        count: data[0].count,
      });
    }

    return data;
  }

  get annotations() {
    return this.args.events.rejectBy('hasCount').map((ev) => ({
      type: ev.error ? 'error' : 'info',
      time: ev.time,
      event: copy(ev),
    }));
  }

  toggleEvent(ev) {
    if (
      this.activeEvent &&
      get(this.activeEvent, 'event.uid') === get(ev, 'event.uid')
    ) {
      this.closeEventDetails();
    } else {
      this.activeEvent = ev;
    }
  }

  closeEventDetails() {
    this.activeEvent = null;
  }
}
