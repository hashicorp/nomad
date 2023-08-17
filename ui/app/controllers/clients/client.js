/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Controller from '@ember/controller';

export default class ClientsClientController extends Controller {
  get client() {
    return this.model;
  }

  get breadcrumb() {
    return {
      title: 'Client',
      label: this.client.get('shortId'),
      args: ['clients.client', this.client.get('id')],
    };
  }
}
