/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { findAll, focus, render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { a11yAudit } from 'ember-a11y-testing/test-support';

module('Integration | Component | app breadcrumbs', function (hooks) {
  setupRenderingTest(hooks);

  const commonCrumbs = [
    { label: 'Jobs', args: ['jobs.index'] },
    { label: 'Job', args: ['jobs.job.index'] },
  ];

  test('header breadcrumbs keep light text, including titled and focused links', async function (assert) {
    await render(hbs`
      <div class="navbar is-secondary">
        <nav class="breadcrumb is-large" aria-label="Breadcrumb">
          <ul><AppBreadcrumbs /></ul>
        </nav>
      </div>
      <Breadcrumb @crumb={{hash label="Jobs" args=(array "jobs.index")}} />
      <Breadcrumb @crumb={{hash title="Job" label="Example" args=(array "jobs.job.index")}} />
    `);

    const links = findAll('[data-test-breadcrumb]');
    for (const link of links) {
      assert.dom(link).hasStyle({ color: 'rgb(255, 255, 255)' });
      await focus(link);
      assert.dom(link).hasStyle({ color: 'rgb(255, 255, 255)' });
    }
    assert.dom('.breadcrumb dt').hasStyle({ color: 'rgb(255, 255, 255)' });
    assert.dom('.breadcrumb dd').hasStyle({ color: 'rgb(255, 255, 255)' });
    await a11yAudit('.navbar.is-secondary', {
      rules: { 'color-contrast': { enabled: true } },
    });
  });

  test('every breadcrumb is rendered correctly', async function (assert) {
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
        'We register the default breadcrumb component if no type is specified on the crumb',
      );

    const renderedCrumbs = findAll('[data-test-breadcrumb]');

    renderedCrumbs.forEach((crumb, index) => {
      assert.deepEqual(
        crumb.textContent.trim(),
        commonCrumbs[index].label,
        `Crumb ${index} is ${commonCrumbs[index].label}`,
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
        'All crumbs without a type render as default breadcrumbs',
      );
  });
});
