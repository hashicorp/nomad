import { assign } from '@ember/polyfills';
import hbs from 'htmlbars-inline-precompile';
import { findAll, find, click, render } from '@ember/test-helpers';
import sinon from 'sinon';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-page/parts/children', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  });

  hooks.afterEach(function() {
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
        gotoJob: () => {},
      },
      options
    );

  test('lists each child', async function(assert) {
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

  test('eventually paginates', async function(assert) {
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
        @gotoJob={{gotoJob}} />
    `);

    const childrenCount = parent.get('children.length');
    assert.ok(childrenCount > 10, 'Parent has more children than one page size');
    assert.equal(findAll('[data-test-job-name]').length, 10, 'Table length maxes out at 10');
    assert.ok(find('.pagination-next'), 'Next button is rendered');

    assert.ok(
      new RegExp(`1.10.+?${childrenCount}`).test(find('.pagination-numbers').textContent.trim())
    );

    await componentA11yAudit(this.element, assert);
  });

  test('is sorted based on the sortProperty and sortDescending properties', async function(assert) {
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

  test('gotoJob is called when a job row is clicked', async function(assert) {
    const gotoJobSpy = sinon.spy();

    this.server.create('job', 'periodic', {
      id: 'parent',
      childrenCount: 1,
      createAllocations: false,
    });

    await this.store.findAll('job');

    const parent = this.store.peekAll('job').findBy('plainId', 'parent');

    this.setProperties(
      props(parent, {
        gotoJob: gotoJobSpy,
      })
    );

    await render(hbs`
      <JobPage::Parts::Children
        @job={{job}}
        @sortProperty={{sortProperty}}
        @sortDescending={{sortDescending}}
        @currentPage={{currentPage}}
        @gotoJob={{gotoJob}} />
    `);

    await click('tr.job-row');

    assert.ok(
      gotoJobSpy.withArgs(parent.get('children.firstObject')).calledOnce,
      'Clicking the job row calls the gotoJob action'
    );
  });
});
