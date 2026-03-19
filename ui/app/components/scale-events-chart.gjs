/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import JsonViewer from 'nomad-ui/components/json-viewer';
import LineChart from 'nomad-ui/components/line-chart';

export default class ScaleEventsChart extends Component {
  @tracked activeEvent = null;

  get data() {
    const data = this.args.events.filterBy('hasCount').sortBy('time');

    // Extend the domain of the chart to the current time.
    data.push({
      time: new Date(),
      count: data[data.length - 1].count,
    });

    // Make sure the domain of the chart includes the first annotation.
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
    return this.args.events.rejectBy('hasCount').map((ev, index) => ({
      type: ev.error ? 'error' : 'info',
      time: ev.time,
      event: cloneScaleEvent(ev, index),
    }));
  }

  toggleEvent = (ev) => {
    if (this.activeEvent?.event?.uid === ev?.event?.uid) {
      this.closeEventDetails();
    } else {
      this.activeEvent = ev;
    }
  };

  closeEventDetails = () => {
    this.activeEvent = null;
  };

  <template>
    <LineChart
      @timeseries={{true}}
      @xProp="time"
      @yProp="count"
      @data={{this.data}}
    >
      <:svg as |c|>
        <c.Area @curve="stepAfter" @data={{this.data}} />
      </:svg>
      <:after as |c|>
        <c.Tooltip class="is-snappy" as |series datum|>
          <li>
            <span class="label"><span
                class="color-swatch is-primary"
              />{{datum.formattedX}}</span>
            <span class="value">{{datum.formattedY}}</span>
          </li>
        </c.Tooltip>
        <c.VAnnotations
          @annotations={{this.annotations}}
          @key="event.uid"
          @activeAnnotation={{this.activeEvent}}
          @annotationClick={{this.toggleEvent}}
        />
      </:after>
    </LineChart>
    {{#if this.activeEvent}}
      <div data-test-event-details>
        <div class="event">
          <div data-test-type class="type">
            {{#if this.activeEvent.event.error}}
              <HdsIcon @name="x-circle-fill" @color="critical" />
            {{else}}
              <HdsIcon @name="info-fill" @color="faint" />
            {{/if}}
          </div>
          <div>
            <p data-test-timestamp class="timestamp">{{formatMonthTs
                this.activeEvent.event.time
              }}</p>
            <p
              data-test-message
              class="message"
            >{{this.activeEvent.event.message}}</p>
          </div>
        </div>
        <JsonViewer
          @json={{this.activeEvent.event.meta}}
          @fluidHeight={{true}}
        />
      </div>
    {{/if}}
  </template>
}

function cloneValue(value) {
  if (typeof structuredClone === 'function') {
    return structuredClone(value);
  }

  return value == null ? value : JSON.parse(JSON.stringify(value));
}

function cloneScaleEvent(event, index) {
  const fallbackUid = `${+event?.time || 0}:${event?.message || ''}:${index}`;

  return {
    uid: event?.uid ?? fallbackUid,
    error: event?.error,
    time: event?.time,
    message: event?.message,
    meta: cloneValue(event?.meta),
  };
}
