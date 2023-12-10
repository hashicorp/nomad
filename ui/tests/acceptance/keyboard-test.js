/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import {
  click,
  currentURL,
  visit,
  triggerEvent,
  triggerKeyEvent,
  findAll,
} from '@ember/test-helpers';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Layout from 'nomad-ui/tests/pages/layout';
import percySnapshot from '@percy/ember';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import faker from 'nomad-ui/mirage/faker';

module('Acceptance | keyboard', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  module('modal', function () {
    test('Opening and closing shortcuts modal with key commands', async function (assert) {
      faker.seed(1);
      assert.expect(4);
      await visit('/');
      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      await percySnapshot(assert);
      await a11yAudit(assert);
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert.notOk(Layout.keyboard.modalShown);
    });

    test('closing shortcuts modal by clicking dismiss', async function (assert) {
      await visit('/');
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert.dom('button.dismiss').isFocused();
      await click('button.dismiss');
      assert.notOk(Layout.keyboard.modalShown);
    });

    test('closing shortcuts modal by clicking outside', async function (assert) {
      await visit('/');
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      await click('.page-layout');
      assert.notOk(Layout.keyboard.modalShown);
    });
  });

  module('Enable/Disable', function (enableDisableHooks) {
    enableDisableHooks.beforeEach(function () {
      window.localStorage.clear();
    });

    test('Shortcuts work by default and stops working when disabled', async function (assert) {
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing g c'
      );
      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert.dom('[data-test-enable-shortcuts-toggle]').hasClass('is-active');
      await click('[data-test-enable-shortcuts-toggle]');
      assert
        .dom('[data-test-enable-shortcuts-toggle]')
        .doesNotHaveClass('is-active');
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert.notOk(Layout.keyboard.modalShown);
      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/clients`,
        'typing g j did not bring you back to the jobs page, since shortcuts are disabled'
      );
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      await click('[data-test-enable-shortcuts-toggle]');
      assert.dom('[data-test-enable-shortcuts-toggle]').hasClass('is-active');
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'typing g j brings me to the jobs page after re-enabling shortcuts'
      );
    });
  });

  module('Local storage bind/rebind', function (rebindHooks) {
    rebindHooks.beforeEach(function () {
      window.localStorage.clear();
    });

    test('You can rebind shortcuts', async function (assert) {
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing g c'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'j' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'end up on the clients page after typing g j'
      );

      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);

      await click(
        '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'f' });
      triggerEvent('.page-layout', 'keydown', { key: 'l' });
      await triggerEvent('.page-layout', 'keydown', { key: 'Enter' });
      assert
        .dom(
          '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
        )
        .hasText('r o f l');

      assert.equal(
        currentURL(),
        `/jobs`,
        'typing g c does not do anything, since I re-bound the shortcut'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'f' });
      await triggerEvent('.page-layout', 'keydown', { key: 'l' });

      assert.equal(
        currentURL(),
        `/clients`,
        'typing the newly bound shortcut brings me to clients'
      );

      await click(
        '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'n' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'p' });
      triggerEvent('.page-layout', 'keydown', { key: 'e' });
      await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
      assert
        .dom(
          '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
        )
        .hasText(
          'r o f l',
          'text unchanged when I hit escape during recording'
        );

      await click(
        '[data-test-command-label="Go to Clients"] button.reset-to-default'
      );
      assert
        .dom(
          '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
        )
        .hasText('g c', 'Resetting to default rebinds the shortcut');
    });

    test('Rebound shortcuts persist from localStorage', async function (assert) {
      window.localStorage.setItem(
        'keyboard.command.Go to Clients',
        JSON.stringify(['b', 'o', 'o', 'p'])
      );
      await visit('/');

      triggerEvent('.page-layout', 'keydown', { key: 'g' });
      await triggerEvent('.page-layout', 'keydown', { key: 'c' });
      assert.equal(
        currentURL(),
        `/jobs`,
        'After a refresh with a localStorage-found binding, a default key binding doesnt do anything'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'b' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      triggerEvent('.page-layout', 'keydown', { key: 'o' });
      await triggerEvent('.page-layout', 'keydown', { key: 'p' });
      assert.equal(
        currentURL(),
        `/clients`,
        'end up on the clients page after typing the localstorage-bound shortcut'
      );

      assert.notOk(Layout.keyboard.modalShown);
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert
        .dom(
          '[data-test-command-label="Go to Clients"] button[data-test-rebinder]'
        )
        .hasText('b o o p');
    });
  });

  module('Hints', function () {
    test('Hints show up on shift', async function (assert) {
      await visit('/');

      await triggerEvent('.page-layout', 'keydown', { key: 'Shift' });
      assert.equal(
        document.querySelectorAll('[data-test-keyboard-hint]').length,
        7,
        'Shows 7 hints by default'
      );
      await triggerEvent('.page-layout', 'keyup', { key: 'Shift' });

      assert.equal(
        document.querySelectorAll('[data-test-keyboard-hint]').length,
        0,
        'Hints disappear when you release Shift'
      );
    });
  });

  module('Dynamic Nav', function (dynamicHooks) {
    dynamicHooks.beforeEach(async function () {
      server.create('node-pool');
      server.create('node');
    });
    test('Dynamic Table Nav', async function (assert) {
      assert.expect(4);
      server.createList('job', 3, { createRecommendations: true });
      await visit('/jobs');

      await triggerEvent('.page-layout', 'keydown', { key: 'Shift' });
      assert.equal(
        document.querySelectorAll('[data-shortcut="Shift+01"]').length,
        1,
        'First job gets a shortcut hint'
      );
      assert.equal(
        document.querySelectorAll('[data-shortcut="Shift+02"]').length,
        1,
        'Second job gets a shortcut hint'
      );
      assert.equal(
        document.querySelectorAll('[data-shortcut="Shift+03"]').length,
        1,
        'Third job gets a shortcut hint'
      );

      triggerEvent('.page-layout', 'keydown', { key: 'Shift' });
      triggerEvent('.page-layout', 'keydown', { key: '0' });
      await triggerEvent('.page-layout', 'keydown', { key: '1' });

      const clickedJob = server.db.jobs.sortBy('modifyIndex').reverse()[0].id;
      assert.equal(
        currentURL(),
        `/jobs/${clickedJob}@default`,
        'Shift+01 takes you to the first job'
      );
    });
    test('Multi-Table Nav', async function (assert) {
      server.createList('job', 3, { createRecommendations: true });
      await visit(
        `/jobs/${server.db.jobs.sortBy('modifyIndex').reverse()[0].id}@default`
      );
      const numberOfGroups = findAll('.task-group-row').length;
      const numberOfAllocs = findAll('.allocation-row').length;

      await triggerEvent('.page-layout', 'keydown', { key: 'Shift' });
      [...Array(numberOfGroups + numberOfAllocs)].forEach((_, iter) => {
        assert.equal(
          document.querySelectorAll(`[data-shortcut="Shift+0${iter + 1}"]`)
            .length,
          1,
          `Dynamic item #${iter + 1} gets a shortcut hint`
        );
      });
      await triggerEvent('.page-layout', 'keyup', { key: 'Shift' });
    });

    test('Dynamic nav arrows and looping', async function (assert) {
      // Make sure user is a management token so Variables appears, etc.
      let token = server.create('token', { type: 'management' });
      window.localStorage.nomadTokenSecret = token.secretId;
      server.createList('job', 3, { createAllocations: true, type: 'system' });
      const jobID = server.db.jobs.sortBy('modifyIndex').reverse()[0].id;
      await visit(`/jobs/${jobID}@default`);

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.ok(
        currentURL().startsWith(`/jobs/${jobID}@default/definition`),
        'Shift+ArrowRight takes you to the next tab (Definition)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/versions`,
        'Shift+ArrowRight takes you to the next tab (Version)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/allocations`,
        'Shift+ArrowRight takes you to the next tab (Allocations)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/evaluations`,
        'Shift+ArrowRight takes you to the next tab (Evaluations)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/clients`,
        'Shift+ArrowRight takes you to the next tab (Clients)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/services`,
        'Shift+ArrowRight takes you to the next tab (Services)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default/variables`,
        'Shift+ArrowRight takes you to the next tab (Variables)'
      );

      await triggerKeyEvent('.page-layout', 'keydown', 'ArrowRight', {
        shiftKey: true,
      });
      assert.equal(
        currentURL(),
        `/jobs/${jobID}@default`,
        'Shift+ArrowRight takes you to the first tab in the loop'
      );
      window.localStorage.nomadTokenSecret = null; // Reset Token
    });

    test('Region switching', async function (assert) {
      ['Detroit', 'Halifax', 'Phoenix', 'Toronto', 'Windsor'].forEach((id) => {
        server.create('region', { id });
      });

      await visit('/jobs');

      // Regions are in the keynav modal
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      await triggerEvent('.page-layout', 'keydown', { key: '?' });
      assert.ok(Layout.keyboard.modalShown);
      assert
        .dom('[data-test-command-label="Switch to Detroit region"]')
        .exists('First created region is in the modal');

      assert
        .dom('[data-test-command-label="Switch to Windsor region"]')
        .exists('Last created region is in the modal');

      // Triggers a region switch to Halifax
      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      await triggerEvent('.page-layout', 'keydown', { key: '2' });
      assert.ok(
        currentURL().includes('region=Halifax'),
        'r 2 command takes you to the second region'
      );

      // Triggers a region switch to Phoenix
      triggerEvent('.page-layout', 'keydown', { key: 'r' });
      await triggerEvent('.page-layout', 'keydown', { key: '3' });
      assert.ok(
        currentURL().includes('region=Phoenix'),
        'r 3 command takes you to the third region'
      );
    });
  });
});
