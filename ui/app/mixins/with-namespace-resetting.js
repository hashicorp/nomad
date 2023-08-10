/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import Mixin from '@ember/object/mixin';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  system: service(),
  jobsController: controller('jobs'),

  actions: {
    gotoJobs(namespace) {
      // Since the setupController hook doesn't fire when transitioning up the
      // route hierarchy, the two sides of the namespace bindings need to be manipulated
      // in order for the jobs route model to reload.
      this.set('jobsController.jobNamespace', namespace.get('id'));
      this.transitionToRoute('jobs');
    },
  },
});
