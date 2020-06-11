import Controller from '@ember/controller';

export default class VolumesController extends Controller {
  queryParams = [
    {
      volumeNamespace: 'namespace',
    },
  ];

  isForbidden = false;

  volumeNamespace = 'default';
}
