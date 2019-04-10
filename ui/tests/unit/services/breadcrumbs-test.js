import Service from '@ember/service';
import Route from '@ember/routing/route';
import Controller from '@ember/controller';
import { get } from '@ember/object';
import { alias } from '@ember/object/computed';
import RSVP from 'rsvp';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import PromiseObject from 'nomad-ui/utils/classes/promise-object';

const makeRoute = (crumbs, controller = {}) =>
  Route.extend({
    breadcrumbs: crumbs,
    controller: Controller.extend(controller).create(),
  });

module('Unit | Service | Breadcrumbs', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      return this.owner.factoryFor('service:breadcrumbs').create();
    };
  });

  hooks.beforeEach(function() {
    const mockRouter = Service.extend({
      currentRouteName: 'application',
      currentURL: '/',
    });

    this.owner.register('service:router', mockRouter);
    this.router = this.owner.lookup('service:router');

    const none = makeRoute();
    const fixed = makeRoute([{ label: 'Static', args: ['static.index'] }]);
    const manyFixed = makeRoute([
      { label: 'Static 1', args: ['static.index', 1] },
      { label: 'Static 2', args: ['static.index', 2] },
    ]);
    const dynamic = makeRoute(model => [{ label: model, args: ['dynamic.index', model] }], {
      model: 'Label of the Crumb',
    });
    const manyDynamic = makeRoute(
      model => [
        { label: get(model, 'fishOne'), args: ['dynamic.index', get(model, 'fishOne')] },
        { label: get(model, 'fishTwo'), args: ['dynamic.index', get(model, 'fishTwo')] },
      ],
      {
        model: {
          fishOne: 'red',
          fishTwo: 'blue',
        },
      }
    );
    const promise = makeRoute([
      PromiseObject.create({
        promise: RSVP.Promise.resolve({
          label: 'delayed',
          args: ['wait.for.it'],
        }),
      }),
    ]);
    const fromURL = makeRoute(model => [{ label: model, args: ['url'] }], {
      router: this.owner.lookup('service:router'),
      model: alias('router.currentURL'),
    });

    this.owner.register('route:none', none);
    this.owner.register('route:none.more-none', none);
    this.owner.register('route:static', fixed);
    this.owner.register('route:static.many', manyFixed);
    this.owner.register('route:dynamic', dynamic);
    this.owner.register('route:dynamic.many', manyDynamic);
    this.owner.register('route:promise', promise);
    this.owner.register('route:url', fromURL);
  });

  test('when the route hierarchy has no breadcrumbs', function(assert) {
    this.router.set('currentRouteName', 'none');

    const service = this.subject();
    assert.deepEqual(service.get('breadcrumbs'), []);
  });

  test('when the route hierarchy has one segment with static crumbs', function(assert) {
    this.router.set('currentRouteName', 'static');

    const service = this.subject();
    assert.deepEqual(service.get('breadcrumbs'), [{ label: 'Static', args: ['static.index'] }]);
  });

  test('when the route hierarchy has multiple segments with static crumbs', function(assert) {
    this.router.set('currentRouteName', 'static.many');

    const service = this.subject();
    assert.deepEqual(service.get('breadcrumbs'), [
      { label: 'Static', args: ['static.index'] },
      { label: 'Static 1', args: ['static.index', 1] },
      { label: 'Static 2', args: ['static.index', 2] },
    ]);
  });

  test('when the route hierarchy has a function as its breadcrumbs property', function(assert) {
    this.router.set('currentRouteName', 'dynamic');

    const service = this.subject();
    assert.deepEqual(service.get('breadcrumbs'), [
      { label: 'Label of the Crumb', args: ['dynamic.index', 'Label of the Crumb'] },
    ]);
  });

  test('when the route hierarchy has multiple segments with dynamic crumbs', function(assert) {
    this.router.set('currentRouteName', 'dynamic.many');

    const service = this.subject();
    assert.deepEqual(service.get('breadcrumbs'), [
      { label: 'Label of the Crumb', args: ['dynamic.index', 'Label of the Crumb'] },
      { label: 'red', args: ['dynamic.index', 'red'] },
      { label: 'blue', args: ['dynamic.index', 'blue'] },
    ]);
  });

  test('when a route provides a breadcrumb that is a promise, it gets passed through to the template', function(assert) {
    this.router.set('currentRouteName', 'promise');

    const service = this.subject();
    assert.ok(service.get('breadcrumbs.firstObject') instanceof PromiseObject);
  });

  // This happens when transitioning to the current route but with a different model
  // jobs.job.index --> jobs.job.index
  // /jobs/one      --> /jobs/two
  test('when the route stays the same but the url changes, breadcrumbs get recomputed', function(assert) {
    this.router.set('currentRouteName', 'url');

    const service = this.subject();
    assert.deepEqual(
      service.get('breadcrumbs'),
      [{ label: '/', args: ['url'] }],
      'The label is initially / as is the router currentURL'
    );

    this.router.set('currentURL', '/somewhere/else');
    assert.deepEqual(
      service.get('breadcrumbs'),
      [{ label: '/somewhere/else', args: ['url'] }],
      'The label changes with currentURL since it is an alias and a change to currentURL recomputes breadcrumbs'
    );
  });
});
