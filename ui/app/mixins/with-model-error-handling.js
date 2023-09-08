/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Mixin from '@ember/object/mixin';
import notifyError from 'nomad-ui/utils/notify-error';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  model() {
    return this._super(...arguments).catch(notifyError(this));
  },
});
