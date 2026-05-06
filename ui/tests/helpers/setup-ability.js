/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (ability) => (hooks) => {
  hooks.beforeEach(function () {
    this.ability = this.owner.lookup(`ability:${ability}`);
    this.abilities = this.owner.lookup('service:abilities');
  });

  hooks.afterEach(function () {
    delete this.ability;
    delete this.abilities;
  });
};
