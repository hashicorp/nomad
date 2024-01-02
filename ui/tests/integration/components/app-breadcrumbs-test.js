/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */
import { setComponentTemplate } from '@ember/component';
import templateOnlyComponent from '@ember/component/template-only';
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

  test('when we register a crumb with a type property, a dedicated breadcrumb/<type> component renders', async function (assert) {
    const crumbs = [
      { label: 'Jobs', args: ['jobs.index'] },
      { type: 'special', label: 'Job', args: ['jobs.job.index'] },
    ];
    this.set('crumbs', crumbs);

    this.owner.register(
      'component:breadcrumbs/special',
      setComponentTemplate(
        hbs`
        <div data-test-breadcrumb-special>Test</div>
      `,
        templateOnlyComponent()
      )
    );

    await render(hbs`
    <AppBreadcrumbs />
    {{#each this.crumbs as |crumb|}}
      <Breadcrumb @crumb={{hash type=crumb.type label=crumb.label args=crumb.args }} />
    {{/each}}
  `);

    assert
      .dom('[data-test-breadcrumb-special]')
      .exists(
        'We can create a new type of breadcrumb component and AppBreadcrumbs will handle rendering by type'
      );

    assert
      .dom('[data-test-breadcrumb-default]')
      .exists('Default breadcrumb registers if no type is specified');
  });
});
