import Component from '@ember/component';
import { computed } from '@ember/object';

import isIp from 'is-ip';

export default Component.extend({
  tagName: 'li',

  href: computed('network.ip', 'port.Value', function() {
    const ip = this.get('network.ip');
    const port = this.get('port.Value');

    let ipHrefPortion;

    if (isIp.v6(ip)) {
      ipHrefPortion = `[${ip}]`;
    } else {
      ipHrefPortion = ip;
    }

    return `http://${ipHrefPortion}:${port}`;
  }),
});
