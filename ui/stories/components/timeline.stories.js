/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import moment from 'moment';

export default {
  title: 'Components/Timeline',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Timeline</h5>
      <ol class="timeline">
        <li class="timeline-note">
          {{format-date yesterday}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              Object number one
            </div>
          </div>
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              Object number two
            </div>
          </div>
        </li>
        <li class="timeline-note">
          {{format-date today}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              Object number three
            </div>
          </div>
        </li>
      </ol>
      <p class="annotation">Timelines are a combination of objects and notes. Objects compose with boxed sections to create structure.</p>
      <p class="annotation">Timeline notes should be used sparingly when possible. In this example there is a note per day rather than a note per object.</p>
      `,
    context: {
      yesterday: moment().subtract(1, 'd'),
      today: moment(),
    },
  };
};

export let Detailed = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Detailed timeline</h5>
      <ol class="timeline">
        <li class="timeline-note">
          {{format-date today}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="tag is-running">Running</span>
              <span class="bumper-left pair is-faded">
                <span class="term">Stable</span>
                <span class="badge is-light is-faded"><code>a387e243</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span class="tooltip" aria-label="{{format-month-ts (now)}}">{{moment-from-now (now)}}</span>
              </span>
            </div>
          </div>
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="tag is-complete">Complete</span>
              <span class="bumper-left pair is-faded">
                <span class="term">Expired</span>
                <span class="badge is-light is-faded"><code>b3220efb</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span>{{format-month-ts yesterday}}</span>
              </span>
            </div>
          </div>
        </li>
        <li class="timeline-note">
          {{format-date yesterday}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="tag is-error">Failed</span>
              <span class="bumper-left pair is-faded">
                <span class="term">Reverted</span>
                <span class="badge is-light is-faded"><code>fec9218e</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span>{{format-month-ts yesterday}}</span>
              </span>
            </div>
          </div>
        </li>
      </ol>
      `,
    context: {
      yesterday: moment().subtract(1, 'd'),
      today: moment(),
    },
  };
};

export let Toggling = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Toggling timeline objects</h5>
      <ol class="timeline">
        <li class="timeline-note">
          {{format-date today}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="tag is-running">Running</span>
              <span class="bumper-left pair is-faded">
                <span class="term">Stable</span>
                <span class="badge is-light is-faded"><code>a387e243</code></span>
              </span>
              <button
                class="button is-light is-compact pull-right"
                onclick={{action (mut toggle1) (not toggle1)}}>
                {{if toggle1 "Close" "Open"}}
              </button>
            </div>
            {{#if toggle1}}
              <div class="boxed-section-body">
                <p>Some details for the timeline object.</p>
              </div>
            {{/if}}
          </div>
        </li>
        <li class="timeline-note">
          {{format-date yesterday}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="tag is-complete">Complete</span>
              <span class="bumper-left pair is-faded">
                <span class="term">Expired</span>
                <span class="badge is-light is-faded"><code>b3220efb</code></span>
              </span>
              <button
                class="button is-light is-compact pull-right"
                onclick={{action (mut toggle2) (not toggle2)}}>
                {{if toggle2 "Close" "Open"}}
              </button>
            </div>
            {{#if toggle2}}
              <div class="boxed-section-body">
                <p>Some details for the timeline object.</p>
              </div>
            {{/if}}
          </div>
        </li>
      </ol>
      <p class="annotation"></p>
      `,
    context: {
      yesterday: moment().subtract(1, 'd'),
      today: moment(),
    },
  };
};

export let Emphasizing = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Emphasizing timeline objects</h5>
      <ol class="timeline">
        <li class="timeline-note">
          {{format-date today}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="pair is-faded">
                <span class="term">Stable</span>
                <span class="badge is-light is-faded"><code>a387e243</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span class="tooltip" aria-label="{{format-ts (now)}}">{{moment-from-now (now)}}</span>
              </span>
            </div>
          </div>
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head">
              Pay attention here
            </div>
            <div class="boxed-section-body">
              <span class="pair is-faded">
                <span class="term">Expired</span>
                <span class="badge is-light is-faded"><code>b3220efb</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span>{{format-ts yesterday}}</span>
              </span>
            </div>
          </div>
        </li>
        <li class="timeline-note">
          {{format-date yesterday}}
        </li>
        <li class="timeline-object">
          <div class="boxed-section">
            <div class="boxed-section-head is-light">
              <span class="pair is-faded">
                <span class="term">Reverted</span>
                <span class="badge is-light is-faded"><code>fec9218e</code></span>
              </span>
              <span class="bumper-left pair is-faded">
                <span class="term">Submitted</span>
                <span>{{format-ts yesterday}}</span>
              </span>
            </div>
          </div>
        </li>
      </ol>
      <p class="annotation">By using a full boxed-section for an emphasized timeline object, the object takes up more space and gets more visual weight. It also adheres to existing patterns.</p>
      `,
    context: {
      yesterday: moment().subtract(1, 'd'),
      today: moment(),
    },
  };
};
