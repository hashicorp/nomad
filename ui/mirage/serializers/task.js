/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
// @ts-check
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['services', 'schedule'],
});
