/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, findAll, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | breadcrumbs', function (hooks) {
  setupRenderingTest(hooks);

  test('it declaratively renders a list of registered crumbs', async function (assert) {
    this.set('isRegistered', false);
    this.set('toggleCrumb', () => this.set('isRegistered', !this.isRegistered));
    await render(hbs`
      <Breadcrumbs as |bb|>
        <ul>
          {{#each bb as |crumb|}}
            <li data-test-crumb={{crumb.args.crumb}}>{{crumb.args.crumb}}</li>
          {{/each}}
        </ul>
      </Breadcrumbs>
      <button data-test-button type="button" {{on "click" toggleCrumb}}>Toggle</button>
      <Breadcrumb @crumb={{'Zoey'}} />
      {{#if this.isRegistered}}
        <Breadcrumb @crumb={{'Tomster'}} />
      {{/if}}
    `);

    assert
      .dom('[data-test-crumb]')
      .exists({ count: 1 }, 'We register one crumb');
    assert
      .dom('[data-test-crumb]')
      .hasText('Zoey', 'The first registered crumb is Zoey');

    await click('[data-test-button]');
    const crumbs = await findAll('[data-test-crumb]');

    assert
      .dom('[data-test-crumb]')
      .exists({ count: 2 }, 'The second crumb registered successfully');
    assert
      .dom(crumbs[0])
      .hasText(
        'Zoey',
        'Breadcrumbs maintain the order in which they are declared'
      );
    assert
      .dom(crumbs[1])
      .hasText(
        'Tomster',
        'Breadcrumbs maintain the order in which they are declared'
      );

    await click('[data-test-button]');
    assert
      .dom('[data-test-crumb]')
      .exists({ count: 1 }, 'We deregister one crumb');
    assert
      .dom('[data-test-crumb]')
      .hasText(
        'Zoey',
        'Zoey remains in the template after Tomster deregisters'
      );
  });

  test('it can register complex crumb objects', async function (assert) {
    await render(hbs`
      <Breadcrumbs as |bb|>
        <ul>
          {{#each bb as |crumb|}}
            <li data-test-crumb>{{crumb.args.crumb.name}}</li>
          {{/each}}
        </ul>
      </Breadcrumbs>
      <Breadcrumb @crumb={{hash name='Tomster'}} />
    `);

    assert
      .dom('[data-test-crumb]')
      .hasText(
        'Tomster',
        'We can access the registered breadcrumbs in the template'
      );
  });
});
