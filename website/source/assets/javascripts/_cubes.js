var HashiCubes = function() {
  var $cube = $('.cube');
  var $cubes = $('#cubes');

  var ROWS = 4;
  var PADDING = 64;

  function getRadiansForAngle(angle) {
    return angle * (Math.PI/180);
  }

  function animateCubes() {
    $lastCube = $cube;
    previousRowLeft = parseInt($lastCube.css('left'), 10)
    previousRowTop = parseInt($lastCube.css('top'), 10);

    var angle = getRadiansForAngle(30);
    var sin = Math.sin(angle) * PADDING;
    var cos = Math.cos(angle) * PADDING;

    // set up our parent columns
    for(var i = 0; i < ROWS; i++){
      var cube = $lastCube.clone();

      cube.css({
        top: previousRowTop - sin,
        left: previousRowLeft - cos,
      });
      $cubes.prepend(cube);
      $lastCube = cube;
      previousRowLeft = parseInt($lastCube.css('left'), 10)
      previousRowTop = parseInt($lastCube.css('top'), 10)
    }

    // use the parent cubes as starting point for rows
    var $allParentCubes = $('.cube');
    var angle = getRadiansForAngle(150);
    var sin = Math.sin(angle) * PADDING;
    var cos = Math.cos(angle) * PADDING;

    for(var j = ROWS; j > -1 ; j--){
      var $baseCube = $($allParentCubes[j]);

      previousRowLeft = parseInt($baseCube.css('left'), 10)
      previousRowTop = parseInt($baseCube.css('top'), 10)

      for(var n = 0; n < ROWS; n++){
        var cube = $baseCube.clone();
        cube.css({
          top: previousRowTop - sin,
          left: previousRowLeft - cos,
        });

        $cubes.prepend(cube);

        $lastCube = cube;
        previousRowLeft = parseInt($lastCube.css('left'), 10)
        previousRowTop = parseInt($lastCube.css('top'), 10)
      }
    }

    var $all = $('.cube');
    for(var c = 0; c < $all.length; c++){
      (function(index){
        setTimeout(function(){
          var $theCube = $($all[index]);
          $theCube.addClass('in')
        }, 100*c)
      })(c)
    }
  }

  animateCubes();
}

$(document).on('turbolinks:load', HashiCubes);
