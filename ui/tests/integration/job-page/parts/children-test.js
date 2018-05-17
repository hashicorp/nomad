import { getOwner } from '@ember/application';
import { assign } from '@ember/polyfills';
import { run } from '@ember/runloop';
import hbs from 'htmlbars-inline-precompile';
import wait from 'ember-test-helpers/wait';
import { findAll, find, click } from 'ember-native-dom-helpers';
import sinon from 'sinon';
import { test, moduleForComponent } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

moduleForComponent('job-page/parts/children', 'Integration | Component | job-page/parts/children', {
  integration: true,
  beforeEach() {
    window.localStorage.clear();
    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
  },
  afterEach() {
    this.server.shutdown();
    window.localStorage.clear();
  },
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

test('lists each child', function(assert) {
  let parent;

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 3,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      parent = this.store.peekAll('job').findBy('plainId', 'parent');
    });

    this.setProperties(props(parent));

    this.render(hbs`
      {{job-page/parts/children
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      assert.equal(
        findAll('[data-test-job-name]').length,
        parent.get('children.length'),
        'A row for each child'
      );
    });
  });
});

test('eventually paginates', function(assert) {
  let parent;

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 11,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      parent = this.store.peekAll('job').findBy('plainId', 'parent');
    });

    this.setProperties(props(parent));

    this.render(hbs`
      {{job-page/parts/children
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      const childrenCount = parent.get('children.length');
      assert.ok(childrenCount > 10, 'Parent has more children than one page size');
      assert.equal(findAll('[data-test-job-name]').length, 10, 'Table length maxes out at 10');
      assert.ok(find('.pagination-next'), 'Next button is rendered');

      assert.ok(
        new RegExp(`1.10.+?${childrenCount}`).test(find('.pagination-numbers').textContent.trim())
      );
    });
  });
});

test('is sorted based on the sortProperty and sortDescending properties', function(assert) {
  let parent;

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 3,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      parent = this.store.peekAll('job').findBy('plainId', 'parent');
    });

    this.setProperties(props(parent));

    this.render(hbs`
      {{job-page/parts/children
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      const sortedChildren = parent.get('children').sortBy('name');
      const childRows = findAll('[data-test-job-name]');

      sortedChildren.reverse().forEach((child, index) => {
        assert.equal(
          childRows[index].textContent.trim(),
          child.get('name'),
          `Child ${index} is ${child.get('name')}`
        );
      });

      this.set('sortDescending', false);

      sortedChildren.forEach((child, index) => {
        assert.equal(
          childRows[index].textContent.trim(),
          child.get('name'),
          `Child ${index} is ${child.get('name')}`
        );
      });

      return wait();
    });
  });
});

test('gotoJob is called when a job row is clicked', function(assert) {
  let parent;
  const gotoJobSpy = sinon.spy();

  this.server.create('job', 'periodic', {
    id: 'parent',
    childrenCount: 1,
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    run(() => {
      parent = this.store.peekAll('job').findBy('plainId', 'parent');
    });

    this.setProperties(
      props(parent, {
        gotoJob: gotoJobSpy,
      })
    );

    this.render(hbs`
      {{job-page/parts/children
        job=job
        sortProperty=sortProperty
        sortDescending=sortDescending
        currentPage=currentPage
        gotoJob=gotoJob}}
    `);

    return wait().then(() => {
      click('tr.job-row');
      assert.ok(
        gotoJobSpy.withArgs(parent.get('children.firstObject')).calledOnce,
        'Clicking the job row calls the gotoJob action'
      );
    });
  });
});
