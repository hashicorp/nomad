/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class NodeEventSerializer extends ApplicationSerializer {
  attrs = {
    time: 'Timestamp',
  };
}
