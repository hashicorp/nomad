/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Mixin from '@ember/object/mixin';
import notifyError from 'nomad-ui/utils/notify-error';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  model() {
    return this._super(...arguments).catch(notifyError(this));
  },
});
