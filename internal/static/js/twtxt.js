// Creare's 'Implied Consent' EU Cookie Law Banner v:2.4
// Conceived by Robert Kent, James Bavington & Tom Foyster

var dropCookie = true;                      // false disables the Cookie, allowing you to style the banner
var cookieDuration = 14;                    // Number of days before the cookie expires, and the banner reappears
var cookieName = 'complianceCookie';        // Name of our cookie
var cookieValue = 'on';                     // Value of cookie

function createDiv(){
    u("body").prepend('<div id="cookie-law" class="container-fluid"><p>This website uses cookies. By continuing we assume your permission to deploy cookies, as detailed in our <a href="/privacy" rel="nofollow" title="Privacy Policy">privacy policy</a>. <a role="button" href="javascript:void(0);" onclick="removeMe();">Close</a></p></div>');
    createCookie(window.cookieName,window.cookieValue, window.cookieDuration); // Create the cookie
}


function createCookie(name,value,days) {
    if (days) {
        var date = new Date();
        date.setTime(date.getTime()+(days*24*60*60*1000));
        var expires = "; expires="+date.toGMTString();
    }
    else var expires = "";
    if(window.dropCookie) {
        document.cookie = name+"="+value+expires+"; path=/";
    }
}

function checkCookie(name) {
    var nameEQ = name + "=";
    var ca = document.cookie.split(';');
    for(var i=0;i < ca.length;i++) {
        var c = ca[i];
        while (c.charAt(0)==' ') c = c.substring(1,c.length);
        if (c.indexOf(nameEQ) == 0) return c.substring(nameEQ.length,c.length);
    }
    return null;
}

function eraseCookie(name) {
    createCookie(name,"",-1);
}

window.onload = function(){
    if(checkCookie(window.cookieName) != window.cookieValue){
        createDiv();
    }
}

function removeMe(){
	var element = document.getElementById('cookie-law');
	element.parentNode.removeChild(element);
}

function replyTo(e) {
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

function editTweet(e) {
  e.preventDefault();

  var el = u("textarea#text")
  var text = document.getElementById("text");

  el.empty();
  el.text(u(e.target).data("text"));
  el.scroll();

  text.focus();

  var size = el.text().length;

  text.setSelectionRange(size, size);

  u("#tweetForm").attr("method", "PATCH");
}

function deleteTweet(e) {
  e.preventDefault();

  if (confirm("Are you sure you want to delete this tweet? This cannot be undone!")) {
    u("#post").html("<i class=\"icss-spinner icss-pulse\"></i>&nbsp;Deleting...");
    u("#post").attr("disabled", true);
    Twix.ajax({
      type: "DELETE",
      url: u("#tweetForm").attr("action"),
      success: function(data) {
        var hash = u(e.target).data("hash");
        u("#" + hash).remove();
      }
    });
  }
};

u(".reply").on("click", replyTo);
u(".edit").on("click", editTweet);
u(".delete").on("click", deleteTweet);

u("#post").on("click", function(e) {
  e.preventDefault();
  u("#post").html("<i class=\"icss-spinner icss-pulse\"></i>&nbsp;Posting...");
  u("#post").attr("disabled", true);

  if (u("#tweetForm").attr("method") == "PATCH") {
    Twix.ajax({
      type: "PATCH",
      data: serialize(u("#tweetForm").first()),
      url: u("#tweetForm").attr("action"),
      success: function(data, res) {
        window.location = res.getResponseHeader("Location");
      }
    });
  } else {
    u("#tweetForm").first().submit();
  }
});


u('#uploadMedia').on("change", function(e){
    u('#uploadSubmit').removeClass('invisible')
});

u('#uploadForm').handle('submit', async e => {
    e.preventDefault();

    u("#uploadSubmit").html("<i class=\"icss-spinner icss-pulse\"></i>&nbsp;Uploading...");

    const body = new FormData(e.target);
    const data = await fetch('/upload', {
    method: 'POST', body
  }).then(
    res => res.json()
  ).then(data => {
        u('#uploadSubmit').addClass('invisible');

        var el = u("textarea#text")
        var text = document.getElementById("text");

        text.value += " ![](" + data.Path + ") ";
        el.scroll();
        text.focus();

        var size = el.text().length;
        text.setSelectionRange(size, size);
    }).catch((error) => {
        console.error('Error:', error);
    });
});
