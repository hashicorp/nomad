import Controller from '@ember/controller';

export default class ClientsClientController extends Controller {
  get breadcrumbs() {
    const model = this.model;
    if (!model) return [];
    return [
      {
        label: model.get('shortId'),
        args: ['clients.client', model.get('id')],
      },
    ];
  }
}
