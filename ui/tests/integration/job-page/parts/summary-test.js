import { getOwner } from '@ember/application';
import hbs from 'htmlbars-inline-precompile';
import wait from 'ember-test-helpers/wait';
import { find, click } from 'ember-native-dom-helpers';
import { test, moduleForComponent } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';

moduleForComponent('job-page/parts/summary', 'Integration | Component | job-page/parts/summary', {
  integration: true,
  beforeEach() {
    fragmentSerializerInitializer(getOwner(this));
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

test('jobs with children use the children diagram', function(assert) {
  this.server.create('job', 'periodic', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));

    this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

    return wait().then(() => {
      assert.ok(find('[data-test-children-status-bar]'), 'Children status bar found');
      assert.notOk(find('[data-test-allocation-status-bar]'), 'Allocation status bar not found');
    });
  });
});

test('jobs without children use the allocations diagram', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));

    this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

    return wait().then(() => {
      assert.ok(find('[data-test-allocation-status-bar]'), 'Allocation status bar found');
      assert.notOk(find('[data-test-children-status-bar]'), 'Children status bar not found');
    });
  });
});

test('the allocations diagram lists all allocation status figures', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));

    this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

    return wait().then(() => {
      assert.equal(
        find('[data-test-legend-value="queued"]').textContent,
        this.get('job.queuedAllocs'),
        `${this.get('job.queuedAllocs')} are queued`
      );

      assert.equal(
        find('[data-test-legend-value="starting"]').textContent,
        this.get('job.startingAllocs'),
        `${this.get('job.startingAllocs')} are starting`
      );

      assert.equal(
        find('[data-test-legend-value="running"]').textContent,
        this.get('job.runningAllocs'),
        `${this.get('job.runningAllocs')} are running`
      );

      assert.equal(
        find('[data-test-legend-value="complete"]').textContent,
        this.get('job.completeAllocs'),
        `${this.get('job.completeAllocs')} are complete`
      );

      assert.equal(
        find('[data-test-legend-value="failed"]').textContent,
        this.get('job.failedAllocs'),
        `${this.get('job.failedAllocs')} are failed`
      );

      assert.equal(
        find('[data-test-legend-value="lost"]').textContent,
        this.get('job.lostAllocs'),
        `${this.get('job.lostAllocs')} are lost`
      );
    });
  });
});

test('the children diagram lists all children status figures', function(assert) {
  this.server.create('job', 'periodic', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait().then(() => {
    this.set('job', this.store.peekAll('job').get('firstObject'));

    this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

    return wait().then(() => {
      assert.equal(
        find('[data-test-legend-value="queued"]').textContent,
        this.get('job.pendingChildren'),
        `${this.get('job.pendingChildren')} are pending`
      );

      assert.equal(
        find('[data-test-legend-value="running"]').textContent,
        this.get('job.runningChildren'),
        `${this.get('job.runningChildren')} are running`
      );

      assert.equal(
        find('[data-test-legend-value="complete"]').textContent,
        this.get('job.deadChildren'),
        `${this.get('job.deadChildren')} are dead`
      );
    });
  });
});

test('the summary block can be collapsed', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait()
    .then(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));

      this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

      return wait();
    })
    .then(() => {
      click('[data-test-accordion-toggle]');
      return wait();
    })
    .then(() => {
      assert.notOk(find('[data-test-accordion-body]'), 'No accordion body');
      assert.notOk(find('[data-test-legend]'), 'No legend');
    });
});

test('when collapsed, the summary block includes an inline version of the chart', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait()
    .then(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));

      this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

      return wait();
    })
    .then(() => {
      click('[data-test-accordion-toggle]');
      return wait();
    })
    .then(() => {
      assert.ok(find('[data-test-allocation-status-bar]'), 'Allocation bar still existed');
      assert.ok(
        find('.inline-chart [data-test-allocation-status-bar]'),
        'Allocation bar is rendered in an inline-chart container'
      );
    });
});

test('the collapsed/expanded state is persisted to localStorage', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  return wait()
    .then(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));

      this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

      return wait();
    })
    .then(() => {
      assert.notOk(window.localStorage.nomadExpandJobSummary, 'No value in localStorage yet');
      click('[data-test-accordion-toggle]');
      return wait();
    })
    .then(() => {
      assert.equal(
        window.localStorage.nomadExpandJobSummary,
        'false',
        'Value is stored for the collapsed state'
      );
    });
});

test('the collapsed/expanded state from localStorage is used for the initial state when available', function(assert) {
  this.server.create('job', {
    createAllocations: false,
  });

  this.store.findAll('job');

  window.localStorage.nomadExpandJobSummary = 'false';

  return wait()
    .then(() => {
      this.set('job', this.store.peekAll('job').get('firstObject'));

      this.render(hbs`
      {{job-page/parts/summary job=job}}
    `);

      return wait();
    })
    .then(() => {
      assert.ok(find('[data-test-allocation-status-bar]'), 'Allocation bar still existed');
      assert.ok(
        find('.inline-chart [data-test-allocation-status-bar]'),
        'Allocation bar is rendered in an inline-chart container'
      );

      click('[data-test-accordion-toggle]');
      return wait();
    })
    .then(() => {
      assert.equal(
        window.localStorage.nomadExpandJobSummary,
        'true',
        'localStorage value still toggles'
      );
      assert.ok(find('[data-test-accordion-body]'), 'Summary still expands');
    });
});
