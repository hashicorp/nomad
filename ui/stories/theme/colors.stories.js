import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Theme|Colors',
};

export const Colors = () => {
  return {
    template: hbs`
      <FreestylePalette @colorPalette={{nomadTheme}} @title="Nomad Theme" @description="Accent and neutrals." />

      <FreestylePalette @colorPalette={{productColors}} @title="Product Colors" @description="Colors from other HashiCorp products. Often borrowed for alternative accents and color schemes." />

      <FreestylePalette @colorPalette={{emotiveColors}} @title="Emotive Colors" @description="Colors used in conjunction with an emotional response." />
      `,
    context: {
      nomadTheme: [
        {
          name: 'Primary',
          base: '#25ba81',
        },
        {
          name: 'Primary Dark',
          base: '#1d9467',
        },
        {
          name: 'Text',
          base: '#0a0a0a',
        },
        {
          name: 'Link',
          base: '#1563ff',
        },
        {
          name: 'Gray',
          base: '#bbc4d1',
        },
        {
          name: 'Off-white',
          base: '#f5f5f5',
        },
      ],

      productColors: [
        {
          name: 'Consul Pink',
          base: '#ff0087',
        },
        {
          name: 'Consul Pink Dark',
          base: '#c62a71',
        },
        {
          name: 'Packer Blue',
          base: '#1daeff',
        },
        {
          name: 'Packer Blue Dark',
          base: '#1d94dd',
        },
        {
          name: 'Terraform Purple',
          base: '#5c4ee5',
        },
        {
          name: 'Terraform Purple Dark',
          base: '#4040b2',
        },
        {
          name: 'Vagrant Blue',
          base: '#1563ff',
        },
        {
          name: 'Vagrant Blue Dark',
          base: '#104eb2',
        },
        {
          name: 'Nomad Green',
          base: '#25ba81',
        },
        {
          name: 'Nomad Green Dark',
          base: '#1d9467',
        },
        {
          name: 'Nomad Green Darker',
          base: '#16704d',
        },
      ],

      emotiveColors: [
        {
          name: 'Success',
          base: '#23d160',
        },
        {
          name: 'Warning',
          base: '#fa8e23',
        },
        {
          name: 'Danger',
          base: '#c84034',
        },
        {
          name: 'Info',
          base: '#1563ff',
        },
      ],
    },
  };
};
