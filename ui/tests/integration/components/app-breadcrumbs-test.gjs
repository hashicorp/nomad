/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { findAll, render } from '@ember/test-helpers';
import AppBreadcrumbs from 'nomad-ui/components/app-breadcrumbs';
import Breadcrumb from 'nomad-ui/components/breadcrumb';

module('Integration | Component | app breadcrumbs', function (hooks) {
  setupRenderingTest(hooks);

  const commonCrumbs = [
    { label: 'Jobs', args: ['jobs.index'] },
    { label: 'Job', args: ['jobs.job.index'] },
  ];

  test('every breadcrumb is rendered correctly', async function (assert) {
    this.commonCrumbs = commonCrumbs;

    await render(
      <template>
        <AppBreadcrumbs />
        {{#each this.commonCrumbs as |crumb|}}
          <Breadcrumb @crumb={{crumb}} />
        {{/each}}
      </template>,
    );

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
    this.crumbs = [
      { label: 'Jobs', args: ['jobs.index'] },
      { label: 'Job', args: ['jobs.job.index'] },
    ];

    await render(
      <template>
        <AppBreadcrumbs />
        {{#each this.crumbs as |crumb|}}
          <Breadcrumb @crumb={{crumb}} />
        {{/each}}
      </template>,
    );

    assert
      .dom('[data-test-breadcrumb-default]')
      .exists(
        { count: 2 },
        'All crumbs without a type render as default breadcrumbs',
      );
  });

  test('crumbs with type job render the job breadcrumb component', async function (assert) {
    const job = {
      idWithNamespace: 'example@default',
      trimmedName: 'example',
      hasChildren: false,
      belongsTo() {
        return {
          id() {
            return null;
          },
        };
      },
      get() {
        return null;
      },
    };

    this.crumbs = [
      {
        label: 'Job',
        type: 'job',
        args: ['jobs.job.index'],
        job,
      },
    ];

    await render(
      <template>
        <AppBreadcrumbs />
        {{#each this.crumbs as |crumb|}}
          <Breadcrumb @crumb={{crumb}} />
        {{/each}}
      </template>,
    );

    assert
      .dom('[data-test-job-breadcrumb]')
      .exists({ count: 1 }, 'Job breadcrumb is rendered for type=job');
  });
});
