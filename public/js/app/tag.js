// Tag

// 蓝色, 红色怎么存到数据库中? 直接存蓝色

Tag.classes = {
	"蓝色": "label label-blue",
	"红色": "label label-red",
	"绿色": "label label-green",
	"黄色": "label label-yellow",
	"blue": "label label-blue",
	"red": "label label-red",
	"green": "label label-green",
	"yellow": "label label-yellow"
}

// 数据库中统一存En
Tag.mapCn2En = {
	"蓝色": "blue",
	"红色": "red",
	"绿色": "green",
	"黄色": "yellow",
}
Tag.mapEn2Cn = {
	"blue": "蓝色",
	"red": "红色",
	"green": "绿色",
	"yellow": "黄色",
}

// 标签自定义颜色持久化管理
Tag.colorMap = {};
try {
	var savedColors = localStorage.getItem("leanote_tag_colors");
	if (savedColors) {
		Tag.colorMap = JSON.parse(savedColors) || {};
	}
} catch(e) {}

Tag.saveColorMap = function() {
	try {
		localStorage.setItem("leanote_tag_colors", JSON.stringify(Tag.colorMap));
	} catch(e) {}
};

// 获取标签对应的样式类名
Tag.getColorClass = function(text) {
	if (!text) return "";
	var enText = Tag.mapCn2En[text] || text;
	var cnText = Tag.mapEn2Cn[text] || text;
	
	// 从持久化字典中查找
	var colorName = Tag.colorMap[text] || Tag.colorMap[enText] || Tag.colorMap[cnText];
	if (colorName) {
		if (colorName.indexOf("label-") === -1) {
			return "label label-" + colorName;
		}
		return colorName.indexOf("label ") === -1 ? "label " + colorName : colorName;
	}
	
	// 从原生 Tag.classes 查找
	if (Tag.classes[text]) return Tag.classes[text];
	if (Tag.classes[enText]) return Tag.classes[enText];
	if (Tag.classes[cnText]) return Tag.classes[cnText];
	
	return "";
};

Tag.t = $("#tags");

// called by Note
Tag.getTags = function() {
	var tags = [];
	Tag.t.children().each(function(){
		var text = $(this).data('tag');
		// text = text.substring(0, text.length - 1); // 把X去掉
		text = Tag.mapCn2En[text] || text;
		tags.push(text);
	});
	// 需要去重吗? 正常情况下不会重复
	return tags;
}

// called by Note
Tag.clearTags = function() {
	Tag.t.html("");
}

// 设置tags
// called by Note
Tag.renderTags = function(tags) {
	Tag.t.html("");
	if(isEmpty(tags)) {
		return;
	}
	// TODO 重构, 这样不高效
	for(var i = 0; i < tags.length; ++i) {
		var tag = tags[i];
		Tag.appendTag(tag);
	}
}

// tag最初状态
function revertTagStatus() {
	$("#addTagTrigger").show();
	$("#addTagInput").hide();
	// hideTagList();
}

function hideTagList(event) {
	$("#tagDropdown").removeClass("open show"); $("#tagColor").removeClass("show");
	if (event) {
		event.stopPropagation();
	}
}
function showTagList(event) {
	$("#tagDropdown").addClass("open show"); $("#tagColor").addClass("show");
	if (event) {
		event.stopPropagation();
	}
}

// 只读模式下显示tags
// called by Note
Tag.renderReadOnlyTags = function(tags) {
	// 先清空
	$("#noteReadTags").html("");
	if(isEmpty(tags) || (tags.length == 1 && tags[0] == "")) {
		$("#noteReadTags").html(getMsg("noTag"));
		return;
	}
	
	var i = true;
	function getNextDefaultClasses() {
		if (i) {
			i = false;
			return "label label-default";
		} else {
			i = true;
			return "label label-info";
		}
	}
	
	for(var j in tags) {
		var text = tags[j];
		text = Tag.mapEn2Cn[text] || text;
		var classes = Tag.getColorClass(text);
		if(!classes) {
			classes = getNextDefaultClasses();
		}
		var tag = tt('<span class="?">?</span>', classes, trimTitle(text));
		
		$("#noteReadTags").append(tag);
	}
}

// 添加tag
// tag = {classes:"label label-red", text:"红色"}
// tag = life
Tag.appendTag = function(tag, save) {
	var isColor = false;
	var classes, text;
	
	if (typeof tag == "object") {
		classes = tag.classes;
		text = tag.text;
		if(!text) {
			return;
		}
		// 检查传入的 classes 中是否包含特定颜色
		var match = (classes || "").match(/label-(red|blue|yellow|green)/);
		if (match) {
			isColor = true;
			var colorName = match[1];
			Tag.colorMap[text] = colorName;
			var enText = Tag.mapCn2En[text] || text;
			Tag.colorMap[enText] = colorName;
			Tag.saveColorMap();
		}
	} else {
		tag = tag == null ? "" : String(tag).trim();
		text = tag;
		if(!text) {
			return;
		}
		var foundClass = Tag.getColorClass(text);
		if(foundClass) {
			classes = foundClass;
			isColor = true;
		} else {
			classes = "label label-default";
		}
	}
	var rawText = text;
	if(LEA.locale == "zh") {
		text = Tag.mapEn2Cn[text] || text;
		rawText = Tag.mapCn2En[rawText] || rawText;
	}
	tag = tt('<span class="?" data-tag="?">?<i title="' + getMsg("delete") + '">X</i></span>', classes, text, text);

	// 避免重复
	var isExists = false;
	$("#tags").children().each(function() {
		var existingText = ($(this).data('tag') || "").trim();
		if (existingText === text || text + "X" === $(this).text()) {
			$(this).remove();
			isExists = true;
		}
	});

	$("#tags").append(tag);

	hideTagList();

	if (!isColor) {
		reRenderTags();
	}
	
	// 笔记已污染
	if(save) {
		Note.curChangedSaveIt(true, function() {
			if (!isExists) {
				ajaxPost("/tag/updateTag", {tag: rawText}, function(ret) {
					if(reIsOk(ret)) {
						Tag.addTagNav(ret.Item);
					}
				});	
			}
		});
	}
}

// 为了颜色间隔, add, delete时调用
function reRenderTags() {
	var defautClasses = [ "label label-default", "label label-info" ];
	var i = 0;
	$("#tags").children().each(
		function() {
			var thisClasses = $(this).attr("class");
			if (thisClasses == "label label-default"
					|| thisClasses == "label label-info") {
				$(this).removeClass(thisClasses).addClass(
						defautClasses[i % 2]);
				i++;
			}
		});
};

// 删除tag
Tag.removeTag = function($target) {
	var tag = $target.data('tag');
	$target.remove();
	reRenderTags();
	if(LEA.locale == "zh") {
		tag = Tag.mapCn2En[tag] || tag;
	}
	Note.curChangedSaveIt(true, function() {
		return;

		ajaxPost("/tag/updateTag", {tag: tag}, function(ret) {
			if(reIsOk(ret)) {
				Tag.addTagNav(ret.Item);
			}
		});
	});
}; 

//-----------
// 左侧nav en -> cn
Tag.tags = [];
Tag.renderTagNav = function(tags) {
	var me = this;
	tags = tags || [];
	Tag.tags = tags;
	$("#tagNav").html('');
	for(var i in tags) {
		var noteTag = tags[i];
		var tag = noteTag.Tag;
		var text = tag;
		if(LEA.locale == "zh") {
			text = Tag.mapEn2Cn[tag] || text;
		}
		text = trimTitle(text);
		if (text) {
			var classes = Tag.getColorClass(tag) || Tag.classes[tag] || "label label-default";
			$("#tagNav").append(tt('<li data-tag="?"><a> <span class="?">?</span> <span class="tag-delete">X</span></li>', tag, classes, text));
		}
	}
};

Tag.deleteTag = function(title) {
	var me = this;
	for(var i = 0; i < this.tags.length; ++i) {
		var tag = this.tags[i];
		if (tag.Tag == title) {
			this.tags.splice(i, 1);
			break;
		}
	}
};

// 添加的标签重新render到左边, 放在第一个位置
// 重新render
Tag.addTagNav = function(newTag) {
	var me = this;
	for(var i in me.tags) {
		var noteTag = me.tags[i];
		if(noteTag.Tag == newTag.Tag) {
			me.tags.splice(i, 1);
			break;
		}
	}
	me.tags.unshift(newTag);
	me.renderTagNav(me.tags);
};

// 当前正在修改颜色的已有标签
Tag.editingTag = null;

// 事件
$(function() {
	// 点击“点击添加标签”
	$("#addTagTrigger").on('click', function(e) {
		e.preventDefault();
		Tag.editingTag = null;
		$(this).hide();
		$("#addTagInput").show().trigger('focus').val("");
		showTagList(e);
	});
	
	$("#addTagInput").on('click focus', function(event) {
		Tag.editingTag = null;
		showTagList(event);
	});
	
	// 在下拉颜色区域及项上阻止 mousedown 默认失焦
	$("#tagColor").on('mousedown', function(event) {
		event.preventDefault();
	});
	$("#tagColor").on('mousedown', 'li, span', function(event) {
		event.preventDefault();
	});
	
	$('#addTagInput').on('keydown', function(e) {
		if (e.keyCode == 13) {
			e.preventDefault();
			var val = ($(this).val() || "").trim();
			if (val) {
				Tag.appendTag(val, true);
				$(this).val("");
			}
			hideTagList();
			$(this).hide();
			$("#addTagTrigger").show();
		} else if (e.keyCode == 27) {
			e.preventDefault();
			hideTagList();
			$(this).val("").hide();
			$("#addTagTrigger").show();
		}
	});
	
	// 点击下拉颜色项：优先将颜色赋予输入框中的文字，只生成一个彩色标签
	$("#tagColor").on('click', 'li', function(event) {
		event.preventDefault();
		event.stopPropagation();
		
		var a = $(this).find("span");
		if(!a.length) {
			a = $(this);
		}
		var colorClass = a.attr("class") || "label label-red";
		var colorText = a.text().trim();
		var inputVal = ($("#addTagInput").val() || "").trim();
		
		if (inputVal) {
			// 用户已在输入框中输入文本，点击颜色：将该颜色应用在输入的标签文本上！只生成这一个标签！
			Tag.appendTag({
				classes: colorClass,
				text: inputVal
			}, true);
			$("#addTagInput").val("");
		} else if (Tag.editingTag) {
			// 为已有标签更新颜色
			Tag.appendTag({
				classes: colorClass,
				text: Tag.editingTag
			}, true);
			Tag.editingTag = null;
		} else {
			// 输入框为空：按传统模式添加以颜色名称命名的标签
			Tag.appendTag({
				classes: colorClass,
				text: colorText
			}, true);
		}
		
		hideTagList();
		$("#addTagInput").hide();
		$("#addTagTrigger").show();
	});
	
	// 点击已有标签文本（非 X 区域），弹出颜色下拉菜单供用户随时修改颜色
	$("#tags").on('click', 'span[data-tag]', function(e) {
		if ($(e.target).is('i')) {
			return; // 点击 X 删除
		}
		e.preventDefault();
		e.stopPropagation();
		var $tagSpan = $(this);
		var currentTag = ($tagSpan.data('tag') || "").trim();
		if (!currentTag) return;
		Tag.editingTag = currentTag;
		showTagList(e);
	});
	
	// 点击页面其他外部区域时，才执行失焦收起
	$(document).on('click', function(e) {
		if (!$(e.target).closest("#tagDropdown, #tags").length) {
			Tag.editingTag = null;
			hideTagList();
			var val = ($("#addTagInput").val() || "").trim();
			if (val) {
				Tag.appendTag(val, true);
				$("#addTagInput").val("");
			}
			$("#addTagInput").hide();
			$("#addTagTrigger").show();
		}
	});
	
	$("#tags").on("click", "i", function() {
		Tag.removeTag($(this).parent());
	});
	//----------
	//
	function deleteTag() {
		$li = $(this).closest('li');
		var tag = ($li.data("tag") || "").trim();
		if(confirm("Are you sure ?")) {
			ajaxPost("/tag/deleteTag", {tag: tag}, function(re) {
				if(reIsOk(re)) {
					var item = re.Item; // 被删除的
					Note.deleteNoteTag(item, tag);
					$li.remove();

					// 删除tags
					Tag.deleteTag(tag);
				}
			});
		};
	}
	
	//-------------
	// nav 标签搜索
	function searchTag() {
		var $li = $(this).closest('li');
		var tag = ($li.data("tag") || "").trim();
		// tag = Tag.mapCn2En[tag] || tag;
		
		// 学习changeNotebook
		
		// 1
		Note.curChangedSaveIt();
		
		// 2 先清空所有
		// 也会把curNoteId清空
		Note.clearAll();
		
		$("#tagSearch").html($li.html()).show();
		$("#tagSearch .tag-delete").remove();
		Note.listIsIn(true, false);
		
		showLoading();
		ajaxGet("/note/searchNoteByTags", {tags: [tag]}, function(notes) {
			hideLoading();
			if(notes) {
				// 和note搜索一样
				// 设空, 防止发生上述情况
				// Note.curNoteId = "";

				Note.renderNotes(notes);
				if(!isEmpty(notes)) {
					Note.changeNote(notes[0].NoteId);
				}
			}
		});
	}
	$("#myTag .folderBody").on("click", "li .label", searchTag);
	// $("#minTagNav").on("click", "li", searchTag);
	
	$("#myTag .folderBody").on("click", "li .tag-delete", deleteTag);
});
