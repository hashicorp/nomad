import Component from '@ember/component';

export default Component.extend({
  options: [
    { name: 'Consul' },
    { name: 'Nomad' },
    { name: 'Packer' },
    { name: 'Terraform' },
    { name: 'Vagrant' },
    { name: 'Vault' },
  ],

  manyOptions: [
    'One',
    'Two',
    'Three',
    'Four',
    'Five',
    'Six',
    'Seven',
    'Eight',
    'Nine',
    'Ten',
    'Eleven',
    'Twelve',
    'Thirteen',
    'Fourteen',
    'Fifteen',
  ].map(name => ({ name })),
});
