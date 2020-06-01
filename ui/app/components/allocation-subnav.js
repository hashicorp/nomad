import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { equal, or } from '@ember/object/computed';

export default Component.extend({
  router: service(),

  tagName: '',

  fsIsActive: equal('router.currentRouteName', 'allocations.allocation.fs'),
  fsRootIsActive: equal('router.currentRouteName', 'allocations.allocation.fs-root'),

  filesLinkActive: or('fsIsActive', 'fsRootIsActive'),
});
