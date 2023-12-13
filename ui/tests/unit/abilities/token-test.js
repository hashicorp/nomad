/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | token', function (hooks) {
  setupTest(hooks);
  setupAbility('token')(hooks);

  test('A non-management user can do nothing with tokens', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
    });
    this.owner.register('service:token', mockToken);
    assert.notOk(this.ability.canRead);
    assert.notOk(this.ability.canList);
    assert.notOk(this.ability.canWrite);
    assert.notOk(this.ability.canUpdate);
    assert.notOk(this.ability.canDestroy);
  });

  test('A management user can do everything with tokens', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });
    this.owner.register('service:token', mockToken);
    assert.ok(this.ability.canRead);
    assert.ok(this.ability.canList);
    assert.ok(this.ability.canWrite);
    assert.ok(this.ability.canUpdate);
    assert.ok(this.ability.canDestroy);
  });

  test('A non-ACL agent (bypassAuthorization) does not allow anything', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });
    this.owner.register('service:token', mockToken);
    assert.notOk(this.ability.canRead);
    assert.notOk(this.ability.canList);
    assert.notOk(this.ability.canWrite);
    assert.notOk(this.ability.canUpdate);
    assert.notOk(this.ability.canDestroy);
  });
});
