/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import hbs from 'htmlbars-inline-precompile';
import { findAll, find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-page/parts/children', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node-pool');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  const props = (job, options = {}) =>
    assign(
      {
        job,
        sortProperty: 'name',
        sortDescending: true,
        currentPage: 1,
      },
      options
    );

  test('lists each child', async function (assert) {
    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 3,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const parent = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(props(parent));

    await render(hbs`
      <JobPage::Parts::Children
        @job={{job}}
        @sortProperty={{sortProperty}}
        @sortDescending={{sortDescending}}
        @currentPage={{currentPage}}
        @gotoJob={{gotoJob}} />
    `);

    assert.equal(
      findAll('[data-test-job-name]').length,
      parent.get('children.length'),
      'A row for each child'
    );
  });

  test('eventually paginates', async function (assert) {
    assert.expect(5);

    const pageSize = 10;
    window.localStorage.nomadPageSize = pageSize;

    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 11,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const parent = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(props(parent));

    await render(hbs`
      <JobPage::Parts::Children
        @job={{job}}
        @sortProperty={{sortProperty}}
        @sortDescending={{sortDescending}}
        @currentPage={{currentPage}}
      />
    `);

    const childrenCount = parent.get('children.length');
    assert.ok(
      childrenCount > pageSize,
      'Parent has more children than one page size'
    );
    assert.equal(
      findAll('[data-test-job-name]').length,
      pageSize,
      'Table length maxes out at 10'
    );
    assert.ok(find('.pagination-next'), 'Next button is rendered');

    assert
      .dom('.pagination-numbers')
      .includesText(
        '1 – 10 of 11',
        'Formats pagination to follow formula `startingIdx - endingIdx of totalTableCount'
      );

    await componentA11yAudit(this.element, assert);
  });

  test('is sorted based on the sortProperty and sortDescending properties', async function (assert) {
    assert.expect(6);

    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 3,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const parent = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(props(parent));

    await render(hbs`
      <JobPage::Parts::Children
        @job={{job}}
        @sortProperty={{sortProperty}}
        @sortDescending={{sortDescending}}
        @currentPage={{currentPage}}
        @gotoJob={{gotoJob}} />
    `);

    const sortedChildren = parent.get('children').sortBy('name');
    const childRows = findAll('[data-test-job-name]');

    sortedChildren.reverse().forEach((child, index) => {
      assert.equal(
        childRows[index].textContent.trim(),
        child.get('name'),
        `Child ${index} is ${child.get('name')}`
      );
    });

    await this.set('sortDescending', false);

    sortedChildren.forEach((child, index) => {
      assert.equal(
        childRows[index].textContent.trim(),
        child.get('name'),
        `Child ${index} is ${child.get('name')}`
      );
    });
  });
});
