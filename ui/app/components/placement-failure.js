import Component from '@ember/component';
import { or } from '@ember/object/computed';

export default Component.extend({
  // Either provide a taskGroup or a failedTGAlloc
  taskGroup: null,
  failedTGAlloc: null,

  placementFailures: or('taskGroup.placementFailures', 'failedTGAlloc'),
});
