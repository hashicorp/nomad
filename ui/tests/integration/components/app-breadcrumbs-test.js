/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { findAll, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | app breadcrumbs', function (hooks) {
  setupRenderingTest(hooks);

  const commonCrumbs = [
    { label: 'Jobs', args: ['jobs.index'] },
    { label: 'Job', args: ['jobs.job.index'] },
  ];

  test('every breadcrumb is rendered correctly', async function (assert) {
    assert.expect(3);
    this.set('commonCrumbs', commonCrumbs);
    await render(hbs`
      <AppBreadcrumbs />
      {{#each this.commonCrumbs as |crumb|}}
        <Breadcrumb @crumb={{hash label=crumb.label args=crumb.args }} />
      {{/each}}
    `);

    assert
      .dom('[data-test-breadcrumb-default]')
      .exists(
        'We register the default breadcrumb component if no type is specified on the crumb'
      );

    const renderedCrumbs = findAll('[data-test-breadcrumb]');

    renderedCrumbs.forEach((crumb, index) => {
      assert.equal(
        crumb.textContent.trim(),
        commonCrumbs[index].label,
        `Crumb ${index} is ${commonCrumbs[index].label}`
      );
    });
  });

  test('crumbs without a type default to the default breadcrumb component', async function (assert) {
    this.set('crumbs', [
      { label: 'Jobs', args: ['jobs.index'] },
      { label: 'Job', args: ['jobs.job.index'] },
    ]);

    await render(hbs`
      <AppBreadcrumbs />
      {{#each this.crumbs as |crumb|}}
        <Breadcrumb @crumb={{hash label=crumb.label args=crumb.args}} />
      {{/each}}
    `);

    assert
      .dom('[data-test-breadcrumb-default]')
      .exists(
        { count: 2 },
        'All crumbs without a type render as default breadcrumbs'
      );
  });
});
