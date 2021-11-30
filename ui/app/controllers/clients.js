import Controller from '@ember/controller';

export default class ClientsController extends Controller {
  isForbidden = false;

  breadcrumbs = [
    {
      label: 'Clients',
      args: ['clients.index'],
    },
  ];
}
