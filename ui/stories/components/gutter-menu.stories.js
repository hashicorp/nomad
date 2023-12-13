/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Gutter Menu',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Gutter menu</h5>
      <div class="columns">
        <div class="column is-4">
          <div class="gutter">
            <aside class="menu">
              <p class="menu-label">Places</p>
              <ul class="menu-list">
                <li><a href="javascript:;" class="is-active">Place One</a></li>
                <li><a href="javascript:;">Place Two</a></li>
              </ul>

              <p class="menu-label">Features</p>
              <ul class="menu-list">
                <li><a href="javascript:;">Feature One</a></li>
                <li><a href="javascript:;">Feature Two</a></li>
              </ul>
            </aside>
          </div>
        </div>
        <div class="column">
          <div class="mock-content">
            <div class="mock-vague"></div>
          </div>
        </div>
      </div>
      `,
  };
};

export let RichComponents = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Gutter navigation with rich components</h5>
      <div class="columns">
        <div class="column is-4">
          <div class="gutter">
            <aside class="menu">
              <p class="menu-label">Places</p>
              <ul class="menu-list">
                <li>
                  <div class="menu-item">
                    <PowerSelect @selected={{or selection "One"}} @options={{array "One" "Two" "Three"}} @onChange={{action (mut selection)}} as |option|>
                      {{option}}
                    </PowerSelect>
                  </div>
                </li>
                <li><a href="javascript:;" class="is-active">Place One</a></li>
                <li><a href="javascript:;">Place Two</a></li>
              </ul>

              <p class="menu-label">Features</p>
              <ul class="menu-list">
                <li><a href="javascript:;">Feature One</a></li>
                <li><a href="javascript:;">Feature Two</a></li>
              </ul>
            </aside>
          </div>
        </div>
        <div class="column">
          <div class="mock-content">
            <div class="mock-vague"></div>
          </div>
        </div>
      </div>
      <p class="annotation">In order to keep the gutter navigation streamlined and easy to navigation, rich components should be avoided when possible. When not possible, they should be kept near the top.</p>
      `,
  };
};

export let ManyItems = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Hypothetical gutter navigation with many items</h5>
      <div class="columns">
        <div class="column is-4">
          <div class="gutter">
            <aside class="menu">
              <p class="menu-label">Places</p>
              <ul class="menu-list">
                {{#each (array "One Two" "Three" "Four" "Five" "Six" "Seven") as |item|}}
                  <li><a href="javascript:;">Place {{item}}</a></li>
                {{/each}}
              </ul>

              <p class="menu-label">Features</p>
              <ul class="menu-list">
                {{#each (array "One Two" "Three" "Four" "Five" "Six" "Seven") as |item|}}
                  <li><a href="javascript:;">Feature {{item}}</a></li>
                {{/each}}
              </ul>

              <p class="menu-label">Other</p>
              <ul class="menu-list">
                <li><a href="javascript:;" class="is-active">The one that didn't fit in</a></li>
              </ul>

              <p class="menu-label">Things</p>
              <ul class="menu-list">
                {{#each (array "One Two" "Three" "Four" "Five" "Six" "Seven") as |item|}}
                  <li><a href="javascript:;">Thing {{item}}</a></li>
                {{/each}}
              </ul>
            </aside>
          </div>
        </div>
        <div class="column">
          <div class="mock-content">
            <div class="mock-vague"></div>
          </div>
        </div>
      </div>
      <p class="annotation">There will only ever be one gutter menu in the Nomad UI, but it helps to imagine a situation where there are many navigation items in the gutter.</p>
      `,
  };
};

export let IconItems = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Hypothetical gutter navigation with icon items</h5>
      <div class="columns">
        <div class="column is-4">
          <div class="gutter">
            <aside class="menu">
              <p class="menu-label">Places</p>
              <ul class="menu-list">
                <li><a href="javascript:;">{{x-icon "clock"}} Place One</a></li>
                <li><a href="javascript:;" class="is-active">{{x-icon "history"}} Place Two</a></li>
              </ul>

              <p class="menu-label">Features</p>
              <ul class="menu-list">
                <li><a href="javascript:;">{{x-icon "alert-triangle"}} Feature One</a></li>
                <li><a href="javascript:;">{{x-icon "media-pause"}} Feature Two</a></li>
              </ul>
            </aside>
          </div>
        </div>
        <div class="column">
          <div class="mock-content">
            <div class="mock-vague"></div>
          </div>
        </div>
      </div>
      <p class="annotation">In the future, the gutter menu may have icons.</p>
      `,
  };
};

export let TaggedItems = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Hypothetical gutter navigation with icon items</h5>
      <div class="columns">
        <div class="column is-4">
          <div class="gutter">
            <aside class="menu">
              <p class="menu-label">Places</p>
              <ul class="menu-list">
                <li><a href="javascript:;">Place One <span class="tag is-small">Beta</span></a></li>
                <li><a href="javascript:;" class="is-active">{{x-icon "history"}} Place Two</a></li>
              </ul>

              <p class="menu-label">Features</p>
              <ul class="menu-list">
                <li><a href="javascript:;">{{x-icon "alert-triangle"}} Feature One</a></li>
                <li><a href="javascript:;">{{x-icon "media-pause"}} Feature Two <span class="tag is-small is-warning">3</span></a></li>
              </ul>
            </aside>
          </div>
        </div>
        <div class="column">
          <div class="mock-content">
            <div class="mock-vague"></div>
          </div>
        </div>
      </div>
      <p class="annotation">Tags can be used to denote beta features or low-priority notifications.</p>
    `,
  };
};

export let Global = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Global gutter navigation</h5>
      <div class="columns">
        <div class="column is-4">
          <GutterMenu>
            {{!-- Page content here --}}
          </GutterMenu>
        </div>
      </div>
      <p class="annotation">Since there will only ever be one gutter menu in the UI, it makes sense to express the menu as a singleton component. This is what that singleton component looks like.</p>
      <p class="annotation"><strong>Note:</strong> Normally the gutter menu is rendered within a page layout and is fixed position. The columns shown in this example are only to imitate the actual width without applying fixed positioning.</p>
      `,
  };
};
