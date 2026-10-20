package models

import (
	errors "errors"
	regexp "regexp"
	strconv "strconv"
	strings "strings"
	unicode "unicode"

	db "rook/server/db"
	encryption "rook/server/encryption"
)

// Avatar choices are allowlists rather than free input: an icon is drawn
// inside every seat badge on every screen, and a fixed set means nobody can
// put a paragraph, a lookalike of someone else's name, or a broken glyph
// there. Adding one is a one-line change.
var Icons = []string{
	"🐦", "🦉", "🦅", "🦆", "🐧", "🦜", "🦚", "🦩", "🦢", "🐓",
	"🦊", "🐺", "🐻", "🐼", "🐨", "🐯", "🦁", "🐸", "🐢", "🐙",
	"🦋", "🐝", "🌵", "🌻", "🍀", "🍄", "⭐", "🌙", "🔥", "⚡",
	"❄️", "🎩", "👑", "🎲", "🃏", "🎯", "🚀", "🎸", "☕", "🍕",
	// Added later. New ones go on the end, so nothing anyone picked moves.
	// No 🤖: that is the computer players' badge.
	"🐱", "🐶", "🐰", "🦝", "🦔", "🐿️", "🦇", "🐳", "🐬", "🦈",
	"🐠", "🦀", "🐌", "🐞", "🦄", "🐲", "🦖", "🐘", "🦒", "🦓",
	"🦘", "🐫", "🐄", "🐷", "🐑", "🐐", "🦌", "🐭", "🐹", "🦦",
	"🌲", "🌴", "🌸", "🌹", "🌷", "🍁", "🌈", "☀️", "🌊", "⛰️",
	"🍎", "🍉", "🍒", "🍩", "🍪", "🧁", "🍔", "🌮", "🍦",
	"♠️", "♥️", "♦️", "♣️", "🎴", "🀄", "🧩", "♟️", "🏆", "🎱",
	"🎨", "🎭", "🎺", "🥁", "📚", "🔔", "💎", "🧭", "⚓", "🛶",
	"🚲", "🏰", "🎈", "🎁", "🪁", "🎃", "🌶️", "🍋", "🥨", "🧀",
	"😎", "🤠", "🥳", "🤓", "😺", "👻", "👽", "🧙", "🦸", "🥷",
	// The rest of the standard emoji set, in Unicode's order: faces,
	// people, nature, food, places, activities, objects, symbols, flags.
	// Default skin tone only; still no 🤖.
	"😀", "😃", "😄", "😁", "😆", "😅", "🤣", "😂", "🙂", "🙃",
	"🫠", "😉", "😊", "😇", "🥰", "😍", "🤩", "😘", "😗", "☺️",
	"😚", "😙", "🥲", "😋", "😛", "😜", "🤪", "😝", "🤑", "🤗",
	"🤭", "🫢", "🫣", "🤫", "🤔", "🫡", "🤐", "🤨", "😐", "😑",
	"😶", "🫥", "😶‍🌫️", "😏", "😒", "🙄", "😬", "😮‍💨", "🤥", "🫨",
	"😌", "😔", "😪", "🤤", "😴", "😷", "🤒", "🤕", "🤢", "🤮",
	"🤧", "🥵", "🥶", "🥴", "😵", "😵‍💫", "🤯", "🥸", "🧐", "😕",
	"🫤", "😟", "🙁", "☹️", "😮", "😯", "😲", "😳", "🥺", "🥹",
	"😦", "😧", "😨", "😰", "😥", "😢", "😭", "😱", "😖", "😣",
	"😞", "😓", "😩", "😫", "🥱", "😤", "😡", "😠", "🤬", "😈",
	"👿", "💀", "☠️", "💩", "🤡", "👹", "👺", "😸", "😹", "😻",
	"😼", "😽", "🙀", "😿", "😾", "🙈", "🙉", "🙊", "💌", "💘",
	"💝", "💖", "💗", "💓", "💞", "💕", "💟", "❣️", "💔", "❤️‍🔥",
	"❤️‍🩹", "❤️", "🩷", "🧡", "💛", "💚", "💙", "🩵", "💜", "🤎",
	"🖤", "🩶", "🤍", "💋", "💯", "💢", "💥", "💫", "💦", "💨",
	"🕳️", "💬", "👁️‍🗨️", "🗨️", "🗯️", "💭", "💤", "👋", "🤚", "🖐️",
	"✋", "🖖", "🫱", "🫲", "🫳", "🫴", "🫷", "🫸", "👌", "🤌",
	"🤏", "✌️", "🤞", "🫰", "🤟", "🤘", "🤙", "👈", "👉", "👆",
	"🖕", "👇", "☝️", "🫵", "👍", "👎", "✊", "👊", "🤛", "🤜",
	"👏", "🙌", "🫶", "👐", "🤲", "🤝", "🙏", "✍️", "💅", "🤳",
	"💪", "🦾", "🦿", "🦵", "🦶", "👂", "🦻", "👃", "🧠", "🫀",
	"🫁", "🦷", "🦴", "👀", "👁️", "👅", "👄", "🫦", "👶", "🧒",
	"👦", "👧", "🧑", "👱", "👨", "🧔", "🧔‍♂️", "🧔‍♀️", "👨‍🦰", "👨‍🦱",
	"👨‍🦳", "👨‍🦲", "👩", "👩‍🦰", "🧑‍🦰", "👩‍🦱", "🧑‍🦱", "👩‍🦳", "🧑‍🦳", "👩‍🦲",
	"🧑‍🦲", "👱‍♀️", "👱‍♂️", "🧓", "👴", "👵", "👼", "🎅", "🤶", "🧑‍🎄",
	"🫅", "🤴", "👸", "👳", "👲", "🧕", "🤵", "👰", "🤰", "🫃",
	"🫄", "🤱", "👩‍🍼", "👨‍🍼", "🧑‍🍼", "💃", "🕺", "🕴️", "👯", "🧑‍🤝‍🧑",
	"👭", "👫", "👬", "💏", "💑", "👪", "🗣️", "👤", "👥", "🫂",
	"👣", "🙍", "🙍‍♂️", "🙍‍♀️", "🙎", "🙎‍♂️", "🙎‍♀️", "🙅", "🙅‍♂️", "🙅‍♀️",
	"🙆", "🙆‍♂️", "🙆‍♀️", "💁", "💁‍♂️", "💁‍♀️", "🙋", "🙋‍♂️", "🙋‍♀️", "🧏",
	"🧏‍♂️", "🧏‍♀️", "🙇", "🙇‍♂️", "🙇‍♀️", "🤦", "🤦‍♂️", "🤦‍♀️", "🤷", "🤷‍♂️",
	"🤷‍♀️", "👮", "👮‍♂️", "👮‍♀️", "🕵️", "🕵️‍♂️", "🕵️‍♀️", "💂", "💂‍♂️", "💂‍♀️",
	"👷", "👷‍♂️", "👷‍♀️", "👳‍♂️", "👳‍♀️", "🤵‍♂️", "🤵‍♀️", "👰‍♂️", "👰‍♀️", "🦸‍♂️",
	"🦸‍♀️", "🦹", "🦹‍♂️", "🦹‍♀️", "🧙‍♂️", "🧙‍♀️", "🧚", "🧚‍♂️", "🧚‍♀️", "🧛",
	"🧛‍♂️", "🧛‍♀️", "🧜", "🧜‍♂️", "🧜‍♀️", "🧝", "🧝‍♂️", "🧝‍♀️", "🧞", "🧞‍♂️",
	"🧞‍♀️", "🧟", "🧟‍♂️", "🧟‍♀️", "💆", "💆‍♂️", "💆‍♀️", "💇", "💇‍♂️", "💇‍♀️",
	"🚶", "🚶‍♂️", "🚶‍♀️", "🧍", "🧍‍♂️", "🧍‍♀️", "🧎", "🧎‍♂️", "🧎‍♀️", "🏃",
	"🏃‍♂️", "🏃‍♀️", "👯‍♂️", "👯‍♀️", "🧖", "🧖‍♂️", "🧖‍♀️", "🧗", "🧗‍♂️", "🧗‍♀️",
	"🏌️", "🏌️‍♂️", "🏌️‍♀️", "🏄", "🏄‍♂️", "🏄‍♀️", "🚣", "🚣‍♂️", "🚣‍♀️", "🏊",
	"🏊‍♂️", "🏊‍♀️", "⛹️", "⛹️‍♂️", "⛹️‍♀️", "🏋️", "🏋️‍♂️", "🏋️‍♀️", "🚴", "🚴‍♂️",
	"🚴‍♀️", "🚵", "🚵‍♂️", "🚵‍♀️", "🤸", "🤸‍♂️", "🤸‍♀️", "🤼", "🤼‍♂️", "🤼‍♀️",
	"🤽", "🤽‍♂️", "🤽‍♀️", "🤾", "🤾‍♂️", "🤾‍♀️", "🤹", "🤹‍♂️", "🤹‍♀️", "🧘",
	"🧘‍♂️", "🧘‍♀️", "🧑‍⚕️", "👨‍⚕️", "👩‍⚕️", "🧑‍🎓", "👨‍🎓", "👩‍🎓", "🧑‍🏫", "👨‍🏫",
	"👩‍🏫", "🧑‍⚖️", "👨‍⚖️", "👩‍⚖️", "🧑‍🌾", "👨‍🌾", "👩‍🌾", "🧑‍🍳", "👨‍🍳", "👩‍🍳",
	"🧑‍🔧", "👨‍🔧", "👩‍🔧", "🧑‍🏭", "👨‍🏭", "👩‍🏭", "🧑‍💼", "👨‍💼", "👩‍💼", "🧑‍🔬",
	"👨‍🔬", "👩‍🔬", "🧑‍💻", "👨‍💻", "👩‍💻", "🧑‍🎤", "👨‍🎤", "👩‍🎤", "🧑‍🎨", "👨‍🎨",
	"👩‍🎨", "🧑‍✈️", "👨‍✈️", "👩‍✈️", "🧑‍🚀", "👨‍🚀", "👩‍🚀", "🧑‍🚒", "👨‍🚒", "👩‍🚒",
	"🧑‍🦯", "👨‍🦯", "👩‍🦯", "🧑‍🦼", "👨‍🦼", "👩‍🦼", "🧑‍🦽", "👨‍🦽", "👩‍🦽", "🧌",
	"🐵", "🐒", "🦍", "🦧", "🐕", "🦮", "🐕‍🦺", "🐩", "🐈", "🐈‍⬛",
	"🐅", "🐆", "🐴", "🫎", "🫏", "🐎", "🦬", "🐮", "🐂", "🐃",
	"🐖", "🐗", "🐽", "🐏", "🐪", "🦙", "🦣", "🐁", "🐀", "🐇",
	"🦫", "🐻‍❄️", "🦥", "🦨", "🦃", "🐔", "🐣", "🐤", "🐥", "🕊️",
	"🐦‍⬛", "🪿", "🦤", "🪶", "🪽", "🐊", "🐍", "🐉", "🦕", "🐋",
	"🐟", "🐡", "🦭", "🐚", "🪸", "🪼", "🦞", "🦐", "🦑", "🦪",
	"🐛", "🐜", "🪲", "🦗", "🪳", "🕷️", "🕸️", "🦂", "🦟", "🪰",
	"🪱", "🦠", "💐", "🪷", "🏵️", "🥀", "🌺", "🌼", "🪻", "🌱",
	"🪴", "🌳", "🌿", "☘️", "🍂", "🍃", "🪹", "🪺", "🍄‍🟫", "🐾",
	"🐦‍🔥", "🍇", "🍈", "🍊", "🍌", "🍍", "🥭", "🍏", "🍐", "🍑",
	"🍓", "🫐", "🥝", "🍅", "🫒", "🥥", "🥑", "🍆", "🥔", "🥕",
	"🌽", "🫑", "🥒", "🥬", "🥦", "🧄", "🧅", "🥜", "🫘", "🌰",
	"🫚", "🫛", "🍋‍🟩", "🍞", "🥐", "🥖", "🫓", "🥯", "🥞", "🧇",
	"🍖", "🍗", "🥩", "🥓", "🍟", "🌭", "🥪", "🌯", "🫔", "🥙",
	"🧆", "🥚", "🍳", "🥘", "🍲", "🫕", "🥣", "🥗", "🍿", "🧈",
	"🧂", "🥫", "🍱", "🍘", "🍙", "🍚", "🍛", "🍜", "🍝", "🍠",
	"🍢", "🍣", "🍤", "🍥", "🥮", "🍡", "🥟", "🥠", "🥡", "🍧",
	"🍨", "🎂", "🍰", "🥧", "🍫", "🍬", "🍭", "🍮", "🍯", "🍼",
	"🥛", "🫖", "🍵", "🍶", "🍾", "🍷", "🍸", "🍹", "🍺", "🍻",
	"🥂", "🥃", "🫗", "🥤", "🧋", "🧃", "🧉", "🧊", "🥢", "🍽️",
	"🍴", "🥄", "🔪", "🫙", "🏺", "🌍", "🌎", "🌏", "🌐", "🗺️",
	"🗾", "🏔️", "🌋", "🗻", "🏕️", "🏖️", "🏜️", "🏝️", "🏞️", "🏟️",
	"🏛️", "🏗️", "🧱", "🪨", "🪵", "🛖", "🏘️", "🏚️", "🏠", "🏡",
	"🏢", "🏣", "🏤", "🏥", "🏦", "🏨", "🏩", "🏪", "🏫", "🏬",
	"🏭", "🏯", "💒", "🗼", "🗽", "⛪", "🕌", "🛕", "🕍", "⛩️",
	"🕋", "⛲", "⛺", "🌁", "🌃", "🏙️", "🌄", "🌅", "🌆", "🌇",
	"🌉", "♨️", "🎠", "🛝", "🎡", "🎢", "💈", "🎪", "🚂", "🚃",
	"🚄", "🚅", "🚆", "🚇", "🚈", "🚉", "🚊", "🚝", "🚞", "🚋",
	"🚌", "🚍", "🚎", "🚐", "🚑", "🚒", "🚓", "🚔", "🚕", "🚖",
	"🚗", "🚘", "🚙", "🛻", "🚚", "🚛", "🚜", "🏎️", "🏍️", "🛵",
	"🦽", "🦼", "🛺", "🛴", "🛹", "🛼", "🚏", "🛣️", "🛤️", "🛢️",
	"⛽", "🛞", "🚨", "🚥", "🚦", "🛑", "🚧", "⛵", "🛥️", "🚤",
	"🛳️", "⛴️", "🚢", "✈️", "🛩️", "🛫", "🛬", "🪂", "💺", "🚁",
	"🚟", "🚠", "🚡", "🛰️", "🛸", "🛎️", "🧳", "⌛", "⏳", "⌚",
	"⏰", "⏱️", "⏲️", "🕰️", "🕛", "🕧", "🕐", "🕜", "🕑", "🕝",
	"🕒", "🕞", "🕓", "🕟", "🕔", "🕠", "🕕", "🕡", "🕖", "🕢",
	"🕗", "🕣", "🕘", "🕤", "🕙", "🕥", "🕚", "🕦", "🌑", "🌒",
	"🌓", "🌔", "🌕", "🌖", "🌗", "🌘", "🌚", "🌛", "🌜", "🌡️",
	"🌝", "🌞", "🪐", "🌟", "🌠", "🌌", "☁️", "⛅", "⛈️", "🌤️",
	"🌥️", "🌦️", "🌧️", "🌨️", "🌩️", "🌪️", "🌫️", "🌬️", "🌀", "🌂",
	"☂️", "☔", "⛱️", "☃️", "⛄", "☄️", "💧", "🎆", "🎇", "🧨",
	"✨", "🎉", "🎊", "🎋", "🎍", "🎎", "🎏", "🎐", "🎑", "🧧",
	"🎀", "🎗️", "🎟️", "🎫", "🎖️", "🥇", "🥈", "🥉", "⚽", "⚾",
	"🥎", "🏀", "🏐", "🏈", "🏉", "🎾", "🥏", "🎳", "🏏", "🏑",
	"🏒", "🥍", "🏓", "🏸", "🥊", "🥋", "🥅", "⛳", "⛸️", "🎣",
	"🤿", "🎽", "🎿", "🛷", "🥌", "🔫", "🎮", "🕹️", "🎰", "🪀",
	"🪅", "🪩", "🪆", "🧸", "🖼️", "🧵", "🪡", "🧶", "🪢", "👓",
	"🕶️", "🥽", "🥼", "🦺", "👔", "👕", "👖", "🧣", "🧤", "🧥",
	"🧦", "👗", "👘", "🥻", "🩱", "🩲", "🩳", "👙", "👚", "🪭",
	"👛", "👜", "👝", "🛍️", "🎒", "🩴", "👞", "👟", "🥾", "🥿",
	"👠", "👡", "🩰", "👢", "🪮", "🧢", "⛑️", "📿", "💄", "💍",
	"🔇", "🔈", "🔉", "🔊", "📢", "📣", "📯", "🔕", "🎼", "🎵",
	"🎶", "🎙️", "🎚️", "🎛️", "🎧", "📻", "🎷", "🪗", "🪕", "🎹",
	"🎻", "🪈", "🪇", "🪘", "📱", "📲", "☎️", "📞", "📟", "📠",
	"🔋", "🪫", "🔌", "💽", "💾", "💿", "📀", "🧮", "🎥", "🎞️",
	"📽️", "🎬", "📺", "📷", "📸", "📹", "📼", "🔍", "🔎", "🕯️",
	"💡", "🔦", "🏮", "🪔", "📔", "📕", "📖", "📗", "📘", "📙",
	"📓", "📒", "📃", "📜", "📄", "📰", "🗞️", "📑", "🔖", "🏷️",
	"💰", "🪙", "💴", "💵", "💶", "💷", "💸", "💳", "🧾", "💹",
	"✉️", "📧", "📨", "📩", "📤", "📥", "📦", "📫", "📪", "📬",
	"📭", "📮", "🗳️", "✏️", "✒️", "🖋️", "🖊️", "🖌️", "🖍️", "📝",
	"💼", "📁", "📂", "🗂️", "📅", "📆", "🗒️", "🗓️", "📇", "📈",
	"📉", "📊", "📋", "📌", "📍", "📎", "🖇️", "📏", "📐", "✂️",
	"🗃️", "🗄️", "🗑️", "🔒", "🔓", "🔏", "🔐", "🔑", "🗝️", "🔨",
	"🪓", "⛏️", "⚒️", "🛠️", "🗡️", "⚔️", "💣", "🪃", "🏹", "🛡️",
	"🪚", "🔧", "🪛", "🔩", "⚙️", "🗜️", "⚖️", "🦯", "🔗", "⛓️",
	"⛓️‍💥", "🪝", "🧰", "🧲", "🪜", "⚗️", "🧪", "🧫", "🧬", "🔬",
	"🔭", "📡", "💉", "🩸", "💊", "🩹", "🩼", "🩺", "🩻", "🚪",
	"🛗", "🪞", "🪟", "🛏️", "🛋️", "🪑", "🚽", "🪠", "🚿", "🛁",
	"🪤", "🪒", "🧴", "🧷", "🧹", "🧺", "🧻", "🪣", "🧼", "🫧",
	"🪥", "🧽", "🧯", "🛒", "🚬", "⚰️", "🪦", "⚱️", "🧿", "🪬",
	"🗿", "🪧", "🪪", "🖥️", "🖨️", "⌨️", "🖱️", "🖲️", "💻", "🏧",
	"🚮", "🚰", "♿", "🚹", "🚺", "🚻", "🚼", "🚾", "🛂", "🛃",
	"🛄", "🛅", "⚠️", "🚸", "⛔", "🚫", "🚳", "🚭", "🚯", "🚱",
	"🚷", "📵", "🔞", "☢️", "☣️", "⬆️", "↗️", "➡️", "↘️", "⬇️",
	"↙️", "⬅️", "↖️", "↕️", "↔️", "↩️", "↪️", "⤴️", "⤵️", "🔃",
	"🔄", "🔙", "🔚", "🔛", "🔜", "🔝", "🛐", "⚛️", "🕉️", "✡️",
	"☸️", "☯️", "✝️", "☦️", "☪️", "☮️", "🕎", "🔯", "🪯", "♈",
	"♉", "♊", "♋", "♌", "♍", "♎", "♏", "♐", "♑", "♒",
	"♓", "⛎", "🔀", "🔁", "🔂", "▶️", "⏩", "⏭️", "⏯️", "◀️",
	"⏪", "⏮️", "🔼", "⏫", "🔽", "⏬", "⏸️", "⏹️", "⏺️", "⏏️",
	"🎦", "🔅", "🔆", "📶", "🛜", "📳", "📴", "♀️", "♂️", "⚧️",
	"✖️", "➕", "➖", "➗", "🟰", "♾️", "‼️", "⁉️", "❓", "❔",
	"❕", "❗", "〰️", "💱", "💲", "⚕️", "♻️", "⚜️", "🔱", "📛",
	"🔰", "⭕", "✅", "☑️", "✔️", "❌", "❎", "➰", "➿", "〽️",
	"✳️", "✴️", "❇️", "©️", "®️", "™️", "#️⃣", "*️⃣", "0️⃣", "1️⃣",
	"2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟", "🔠",
	"🔡", "🔢", "🔣", "🔤", "🅰️", "🆎", "🅱️", "🆑", "🆒", "🆓",
	"ℹ️", "🆔", "Ⓜ️", "🆕", "🆖", "🅾️", "🆗", "🅿️", "🆘", "🆙",
	"🆚", "🈁", "🈂️", "🈷️", "🈶", "🈯", "🉐", "🈹", "🈚", "🈲",
	"🉑", "🈸", "🈴", "🈳", "㊗️", "㊙️", "🈺", "🈵", "🔴", "🟠",
	"🟡", "🟢", "🔵", "🟣", "🟤", "⚫", "⚪", "🟥", "🟧", "🟨",
	"🟩", "🟦", "🟪", "🟫", "⬛", "⬜", "◼️", "◻️", "◾", "◽",
	"▪️", "▫️", "🔶", "🔷", "🔸", "🔹", "🔺", "🔻", "💠", "🔘",
	"🔳", "🔲", "🏁", "🚩", "🎌", "🏴", "🏳️", "🏳️‍🌈", "🏳️‍⚧️", "🏴‍☠️",
	"🇦🇨", "🇦🇩", "🇦🇪", "🇦🇫", "🇦🇬", "🇦🇮", "🇦🇱", "🇦🇲", "🇦🇴", "🇦🇶",
	"🇦🇷", "🇦🇸", "🇦🇹", "🇦🇺", "🇦🇼", "🇦🇽", "🇦🇿", "🇧🇦", "🇧🇧", "🇧🇩",
	"🇧🇪", "🇧🇫", "🇧🇬", "🇧🇭", "🇧🇮", "🇧🇯", "🇧🇱", "🇧🇲", "🇧🇳", "🇧🇴",
	"🇧🇶", "🇧🇷", "🇧🇸", "🇧🇹", "🇧🇻", "🇧🇼", "🇧🇾", "🇧🇿", "🇨🇦", "🇨🇨",
	"🇨🇩", "🇨🇫", "🇨🇬", "🇨🇭", "🇨🇮", "🇨🇰", "🇨🇱", "🇨🇲", "🇨🇳", "🇨🇴",
	"🇨🇵", "🇨🇷", "🇨🇺", "🇨🇻", "🇨🇼", "🇨🇽", "🇨🇾", "🇨🇿", "🇩🇪", "🇩🇬",
	"🇩🇯", "🇩🇰", "🇩🇲", "🇩🇴", "🇩🇿", "🇪🇦", "🇪🇨", "🇪🇪", "🇪🇬", "🇪🇭",
	"🇪🇷", "🇪🇸", "🇪🇹", "🇪🇺", "🇫🇮", "🇫🇯", "🇫🇰", "🇫🇲", "🇫🇴", "🇫🇷",
	"🇬🇦", "🇬🇧", "🇬🇩", "🇬🇪", "🇬🇫", "🇬🇬", "🇬🇭", "🇬🇮", "🇬🇱", "🇬🇲",
	"🇬🇳", "🇬🇵", "🇬🇶", "🇬🇷", "🇬🇸", "🇬🇹", "🇬🇺", "🇬🇼", "🇬🇾", "🇭🇰",
	"🇭🇲", "🇭🇳", "🇭🇷", "🇭🇹", "🇭🇺", "🇮🇨", "🇮🇩", "🇮🇪", "🇮🇱", "🇮🇲",
	"🇮🇳", "🇮🇴", "🇮🇶", "🇮🇷", "🇮🇸", "🇮🇹", "🇯🇪", "🇯🇲", "🇯🇴", "🇯🇵",
	"🇰🇪", "🇰🇬", "🇰🇭", "🇰🇮", "🇰🇲", "🇰🇳", "🇰🇵", "🇰🇷", "🇰🇼", "🇰🇾",
	"🇰🇿", "🇱🇦", "🇱🇧", "🇱🇨", "🇱🇮", "🇱🇰", "🇱🇷", "🇱🇸", "🇱🇹", "🇱🇺",
	"🇱🇻", "🇱🇾", "🇲🇦", "🇲🇨", "🇲🇩", "🇲🇪", "🇲🇫", "🇲🇬", "🇲🇭", "🇲🇰",
	"🇲🇱", "🇲🇲", "🇲🇳", "🇲🇴", "🇲🇵", "🇲🇶", "🇲🇷", "🇲🇸", "🇲🇹", "🇲🇺",
	"🇲🇻", "🇲🇼", "🇲🇽", "🇲🇾", "🇲🇿", "🇳🇦", "🇳🇨", "🇳🇪", "🇳🇫", "🇳🇬",
	"🇳🇮", "🇳🇱", "🇳🇴", "🇳🇵", "🇳🇷", "🇳🇺", "🇳🇿", "🇴🇲", "🇵🇦", "🇵🇪",
	"🇵🇫", "🇵🇬", "🇵🇭", "🇵🇰", "🇵🇱", "🇵🇲", "🇵🇳", "🇵🇷", "🇵🇸", "🇵🇹",
	"🇵🇼", "🇵🇾", "🇶🇦", "🇷🇪", "🇷🇴", "🇷🇸", "🇷🇺", "🇷🇼", "🇸🇦", "🇸🇧",
	"🇸🇨", "🇸🇩", "🇸🇪", "🇸🇬", "🇸🇭", "🇸🇮", "🇸🇯", "🇸🇰", "🇸🇱", "🇸🇲",
	"🇸🇳", "🇸🇴", "🇸🇷", "🇸🇸", "🇸🇹", "🇸🇻", "🇸🇽", "🇸🇾", "🇸🇿", "🇹🇦",
	"🇹🇨", "🇹🇩", "🇹🇫", "🇹🇬", "🇹🇭", "🇹🇯", "🇹🇰", "🇹🇱", "🇹🇲", "🇹🇳",
	"🇹🇴", "🇹🇷", "🇹🇹", "🇹🇻", "🇹🇼", "🇹🇿", "🇺🇦", "🇺🇬", "🇺🇲", "🇺🇳",
	"🇺🇸", "🇺🇾", "🇺🇿", "🇻🇦", "🇻🇨", "🇻🇪", "🇻🇬", "🇻🇮", "🇻🇳", "🇻🇺",
	"🇼🇫", "🇼🇸", "🇽🇰", "🇾🇪", "🇾🇹", "🇿🇦", "🇿🇲", "🇿🇼", "🏴󠁧󠁢󠁥󠁮󠁧󠁿", "🏴󠁧󠁢󠁳󠁣󠁴󠁿",
	"🏴󠁧󠁢󠁷󠁬󠁳󠁿",
}

// Colors are names, not hex values, so the frontend picks tones that work in
// both themes. Besides these, a color may be a custom hue, "h0" to "h359":
// still only a hue, so the frontend's saturation and lightness keep it
// readable in both themes too.
var Colors = []string{ "teal" , "coral" , "plum" , "gold" , "forest" , "sky" , "rose" , "berry" }

var huePattern = regexp.MustCompile( `^h(0|[1-9][0-9]{0,2})$` )

// ValidColor accepts "", a named color, or a custom hue.
func ValidColor( color string ) ( ok bool ) {
	if validChoice( color , Colors ) {
		ok = true
		return
	}
	match := huePattern.FindStringSubmatch( color )
	if match == nil { return }
	hue , err := strconv.Atoi( match[ 1 ] )
	ok = err == nil && hue < 360
	return
}

var ErrBadProfile = errors.New( "unknown icon or color" )
var ErrBadPin = errors.New( "pin must be 4 to 8 digits" )

func validChoice( value string , choices []string ) ( ok bool ) {
	if value == "" {
		ok = true
		return
	}
	for _ , choice := range choices {
		if choice == value {
			ok = true
			return
		}
	}
	return
}

// SetUserAvatar saves an icon and color; empty strings go back to initials
// on an automatic color.
func SetUserAvatar( store *db.Store , user_id uint64 , icon string , color string ) ( err error ) {
	if validChoice( icon , Icons ) == false || ValidColor( color ) == false {
		err = ErrBadProfile
		return
	}
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			user := item.( *User )
			user.Icon = icon
			user.Color = color
			return
		} )
	return
}

func SetUserCanHint( store *db.Store , user_id uint64 , can_hint bool ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).CanHint = can_hint
			return
		} )
	return
}

// SetUserBotLevel saves the player's default bot level. The route checks the
// name against game.Levels.
func SetUserBotLevel( store *db.Store , user_id uint64 , level string ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).BotLevel = level
			return
		} )
	return
}

// SetUserAutoPlayLevel saves the level auto play uses for the player. The
// route checks the name against game.Levels.
func SetUserAutoPlayLevel( store *db.Store , user_id uint64 , level string ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).AutoPlayLevel = level
			return
		} )
	return
}

func SetUserCanUndo( store *db.Store , user_id uint64 , can_undo bool ) ( err error ) {
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).CanUndo = can_undo
			return
		} )
	return
}

var pinPattern = regexp.MustCompile( `^[0-9]{4,8}$` )

// SetUserPin sets or (with "") clears the PIN. It is bcrypt-hashed like a
// login link: four digits is a small space, so the hash is only half the
// protection -- the claim route also limits attempts per account.
func SetUserPin( store *db.Store , user_id uint64 , pin string ) ( err error ) {
	hash := ""
	if pin != "" {
		if pinPattern.MatchString( pin ) == false {
			err = ErrBadPin
			return
		}
		if hash , err = encryption.BcryptHash( pin ); err != nil { return }
	}
	err = store.UpdateValue( db.BucketUsers , encryption.Uint64ToBytes( user_id ) ,
		func() any { return &User{} } ,
		func( item any ) ( mutate_err error ) {
			item.( *User ).PinHash = hash
			return
		} )
	return
}

func ( user *User ) HasPin() ( result bool ) {
	result = user.PinHash != ""
	return
}

func ( user *User ) CheckPin( pin string ) ( ok bool ) {
	if user.PinHash == "" {
		ok = true
		return
	}
	ok = pin != "" && encryption.BcryptCompare( user.PinHash , pin )
	return
}

// normalizeName folds a name for comparison: case, spacing, and
// punctuation should not make "Anna-Marie" a stranger to "anna marie".
func normalizeName( name string ) ( result string ) {
	var builder strings.Builder
	for _ , character := range strings.ToLower( strings.TrimSpace( name ) ) {
		if unicode.IsLetter( character ) || unicode.IsDigit( character ) {
			builder.WriteRune( character )
		}
	}
	result = builder.String()
	return
}

func editDistance( a string , b string ) ( distance int ) {
	left , right := []rune( a ) , []rune( b )
	previous := make( []int , len( right )+1 )
	for index := range previous {
		previous[ index ] = index
	}
	for i := 1; i <= len( left ); i++ {
		current := make( []int , len( right )+1 )
		current[ 0 ] = i
		for j := 1; j <= len( right ); j++ {
			cost := 1
			if left[ i-1 ] == right[ j-1 ] { cost = 0 }
			current[ j ] = min( previous[ j ]+1 , current[ j-1 ]+1 , previous[ j-1 ]+cost )
		}
		previous = current
	}
	distance = previous[ len( right ) ]
	return
}

// NameCloseness scores how alike two names are: 0 is the same name once
// case and punctuation are ignored, larger is further apart, and -1 means
// "not similar enough to suggest". This is the "did you mean...?" behind
// sign-up links.
func NameCloseness( typed string , existing string ) ( score int ) {
	a , b := normalizeName( typed ) , normalizeName( existing )
	if a == "" || b == "" {
		score = -1
		return
	}
	if a == b { return }
	if len( a ) >= 3 && ( strings.HasPrefix( b , a ) || strings.HasPrefix( a , b ) ) {
		score = 1
		return
	}
	distance := editDistance( a , b )
	allowed := 1
	if len( []rune( b ) ) >= 6 { allowed = 2 }
	if distance > allowed {
		score = -1
		return
	}
	score = distance + 1
	return
}
