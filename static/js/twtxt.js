function reply(e) {
  e.preventDefault();

  var el = u("textarea#text")
  var text = document.getElementById("text");

  el.empty();
  el.text(u(e.target).data("reply"));
  el.scroll();

  text.focus();

  var size = el.text().length;

  text.setSelectionRange(size, size);
}

u(".reply").on("click", reply);

function readURL(input) {
	if (input.files && input.files[0]) {
		var reader = new FileReader();

		reader.onload = function (e) {
			u('img.profile-pic').attr('src', e.target.result);
		}

		reader.readAsDataURL(input.files[0]);
	}
}

u("input.file-upload").on('change', function(){
	readURL(this);
});

u("div.upload-button").on('click', function() {
	u("input.file-upload").click();
});