package youtube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/dop251/goja"
)

func (c *Client) decipherURL(ctx context.Context, videoID string, cipher string) (string, error) {
	params, err := url.ParseQuery(cipher)
	if err != nil {
		return "", err
	}

	uri, err := url.Parse(params.Get("url"))
	if err != nil {
		return "", err
	}
	query := uri.Query()

	config, err := c.getPlayerConfig(ctx, videoID)
	if err != nil {
		return "", err
	}

	// decrypt s-parameter
	bs, err := config.decrypt([]byte(params.Get("s")))
	if err != nil {
		return "", err
	}
	query.Add(params.Get("sp"), string(bs))

	query, err = c.decryptNParam(config, query)
	if err != nil {
		return "", err
	}

	uri.RawQuery = query.Encode()

	return uri.String(), nil
}

// see https://github.com/kkdai/youtube/pull/244
func (c *Client) unThrottle(ctx context.Context, videoID string, urlString string) (string, error) {
	config, err := c.getPlayerConfig(ctx, videoID)
	if err != nil {
		return "", err
	}

	uri, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}

	// for debugging
	if artifactsFolder != "" {
		writeArtifact("video-"+videoID+".url", []byte(uri.String()))
	}

	query, err := c.decryptNParam(config, uri.Query())
	if err != nil {
		return "", err
	}

	uri.RawQuery = query.Encode()
	return uri.String(), nil
}

func (c *Client) decryptNParam(config playerConfig, query url.Values) (url.Values, error) {
	// decrypt n-parameter
	nSig := query.Get("v")
	log := Logger.With("n", nSig)

	if nSig != "" {
		nDecoded, err := config.decodeNsig(nSig)
		if err != nil {
			return nil, fmt.Errorf("unable to decode nSig: %w", err)
		}
		query.Set("v", nDecoded)
		log = log.With("decoded", nDecoded)
	}

	log.Debug("nParam")

	return query, nil
}

const (
	jsvarStr   = "[a-zA-Z_\\$][a-zA-Z_0-9]*"
	reverseStr = ":function\\(a\\)\\{" +
		"(?:return )?a\\.reverse\\(\\)" +
		"\\}"
	spliceStr = ":function\\(a,b\\)\\{" +
		"a\\.splice\\(0,b\\)" +
		"\\}"
	swapStr = ":function\\(a,b\\)\\{" +
		"var c=a\\[0\\];a\\[0\\]=a\\[b(?:%a\\.length)?\\];a\\[b(?:%a\\.length)?\\]=c(?:;return a)?" +
		"\\}"
)

var (
	nFunctionNameRegexp = regexp.MustCompile("\\.get\\(\"n\"\\)\\)&&\\(b=([a-zA-Z0-9$]{0,3})\\[(\\d+)\\](.+)\\|\\|([a-zA-Z0-9]{0,3})")
	actionsObjRegexp    = regexp.MustCompile(fmt.Sprintf(
		"var (%s)=\\{((?:(?:%s%s|%s%s|%s%s),?\\n?)+)\\};", jsvarStr, jsvarStr, swapStr, jsvarStr, spliceStr, jsvarStr, reverseStr))

	actionsFuncRegexp = regexp.MustCompile(fmt.Sprintf(
		"function(?: %s)?\\(a\\)\\{"+
			"a=a\\.split\\(\"\"\\);\\s*"+
			"((?:(?:a=)?%s\\.%s\\(a,\\d+\\);)+)"+
			"return a\\.join\\(\"\"\\)"+
			"\\}", jsvarStr, jsvarStr, jsvarStr))

	reverseRegexp = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, reverseStr))
	spliceRegexp  = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, spliceStr))
	swapRegexp    = regexp.MustCompile(fmt.Sprintf("(?m)(?:^|,)(%s)%s", jsvarStr, swapStr))
)

func (config playerConfig) decodeNsig(encoded string) (string, error) {
	//fmt.Println(encoded)
	//fBody, err := config.getNFunction()
	//if err != nil {
	//	return "", err
	//}
	fBody := `function(a){var b=a.split(a.slice(0,0)),c=[-1762978610,224018618,372078021,-1584087523,1631548760,-1523401041,function(d,e){e.length!=0&&(d=(d%e.length+e.length)%e.length,e.splice(0,1,e.splice(d,1,e[0])[0]))},
	   function(){for(var d=64,e=[];++d-e.length-32;)switch(d){case 58:d=96;continue;case 91:d=44;break;case 65:d=47;continue;case 46:d=153;case 123:d-=58;default:e.push(String.fromCharCode(d))}return e},
	   -1875214889,-1221130857,1929975707,-1762978610,-1987230004,1562453898,function(){for(var d=64,e=[];++d-e.length-32;){switch(d){case 91:d=44;continue;case 123:d=65;break;case 65:d-=18;continue;case 58:d=96;continue;case 46:d=95}e.push(String.fromCharCode(d))}return e},
	   -460348276,1141163327,-1209828986,642821393,1673505824,function(d,e){e=(e%d.length+d.length)%d.length;d.splice(-e).reverse().forEach(function(f){d.unshift(f)})},
	   1562453898,1039712780,198668272,733991645,/[;]'([,/,59,/]){/,-1775324511,-1662739629,-331919071,-504717341,function(d,e){if(e.length!=0){d=(d%e.length+e.length)%e.length;var f=e[0];e[0]=e[d];e[d]=f}},
	   -1514776029,-2107135749,function(d,e,f,h,l,m){return e(h,l,m)},
	   null,function(d,e){for(e=(e%d.length+d.length)%d.length;e--;)d.unshift(d.pop())},
	   function(d,e){d=(d%e.length+e.length)%e.length;e.splice(d,1)},
	   1994577881,function(d,e,f){var h=d.length;f.forEach(function(l,m,n){this.push(n[m]=d[(d.indexOf(l)-d.indexOf(this[m])+m+h--)%d.length])},e.split(""))},
	   1581593411,-81352162,-393381805,-1818317157,1582095365,-1669922702,-1501675767,function(d){for(var e=d.length;e;)d.push(d.splice(--e,1)[0])},
	   283239203,2026246890,"pop",438638805,function(){for(var d=64,e=[];++d-e.length-32;)switch(d){case 46:d=95;default:e.push(String.fromCharCode(d));case 94:case 95:case 96:break;case 123:d-=76;case 92:case 93:continue;case 58:d=44;case 91:}return e},
	   1885980169,function(d,e,f,h,l){return e(f,h,l)},
	   -1325120929,1598665835,1852467764,null,-998626056,-1916462760,b,null,42490944,-367416562,-1334988290,1212718012,b,424069788,function(d){d.reverse()},
	   -331919071,1145271056,1410038943,-886548250,1791824832,-418171758,function(){for(var d=64,e=[];++d-e.length-32;){switch(d){case 58:d-=14;case 91:case 92:case 93:continue;case 123:d=47;case 94:case 95:case 96:continue;case 46:d=95}e.push(String.fromCharCode(d))}return e},
	   394991787,189178273,/(\)],);[\]]/,b,-861027642,938992366,1617953070,-1390903909,-2068775914,1776751202];c[34]=c;c[57]=c;c[61]=c;try{try{c[24]>10&&((((0,c[53])((0,c[6])(c[85],c[60]),c[6],c[44],c[79]),c[20])(c[61],c[82]),c[17])(c[16],c[44]),1)||((((0,c[52])(c[24],c[80]),c[82])(c[41],c[21]),c[82])(c[25],c[26]),c[82])(c[40],c[20]),c[new Date("1969-12-31T18:46:01.000-05:15")/1E3]!=-8&&(c[30]==4&&(((((0,c[84])((0,c[53])(),c[9],c[39]),c[13])((0,c[13])((0,c[66])(c[20],c[19]),c[84],(0,c[35])(),c[9],c[39]),
	       c[81],c[80],c[62]),((((0,c[82])(c[54],c[39]),c[28])(c[80]),(0,c[76])(c[70],c[21]),c[84])((0,c[11])(),c[9],c[20]),c[28])(c[21]),c[28])(c[39]),((0,c[82])(c[47],c[26]),c[84])((0,c[35])(),c[9],c[26]),c[82])(c[68],c[39]),[])||((((((0,c[13])(((0,c[81])(c[20],c[34]),c[76])(c[73],c[80]),c[82],c[49],c[39]),c[81])(c[new Date("1970-01-01T08:29:50.000+08:30")/1E3+49],c[75]),c[76])(c[14],c[39]),c[76])(c[83],c[80]),c[81])(c[39],c[224+new Date("1970-01-01T03:42:21.000+03:45")/1E3]),c[84])((0,c[60])(),c[9],c[26])<=
	   (0,c[79])((0,c[66])(c[26],c[37]),c[76],((0,c[13])((0,c[82])(c[50],c[39]),c[82],c[72],c[20]),c[76])(c[27],c[21]),c[77],c[80])),c[40]<-7&&(((0,c[76])(c[36],c[0]),(0,c[66])(c[17],c[44]),c[38])(c[75]),"null")||((0,c[62])((0,c[69])(c[55]),c[85],c[14],c[36]),c[79])(c[6],c[55]),c[48]!=-6&&(c[68]<=6&&(((0,c[79])(c[19],c[81]),c[85])(c[16],c[49]),1)||((0,c[47])(c[58]),c[0])(c[68],c[49])),c[new Date("1969-12-31T22:30:10.000-01:30")/1E3]!==0&&(c[47]!==9||(((((0,c[8])((0,c[63])(),c[19],c[36]),c[8])((0,c[63])(),
	   c[19],c[30]),(((0,c[6])(c[14],c[30]),c[6])(c[17],c[36]),c[6])(c[42],c[4]),c[8])((0,c[21])(),c[19],c[36]),c[16])(c[4]),0))&&(((((0,c[5])(c[27],c[22]),(0,c[59])(c[64],c[1]),(0,c[59])(c[50],c[33]),c[83])(c[17],c[72]),c[83])(c[25],c[1]),c[73])(c[27],c[85]),c[2])(c[72],c[37])}catch(d){(0,c[new Date("1970-01-01T06:15:39.000+06:15")/1E3])(c[59]),(0,c[46])((0,c[85])(c[66],c[59])*(0,c[85])(c[49],c[12]),c[28],c[53],c[38]),(0,c[29])(c[new Date("1970-01-01T06:00:11.000+06:00")/1E3],c[59])}try{c[17]!=-8?(0,c[46])((((0,c[28])(c[72],
	   c[45]),c[23])(c[48],c[50]),c[13])(c[27],c[56]),c[67],c[81],c[64]):((((0,c[61])(c[40],c[35]),c[85])(c[60],c[62]),c[7])((0,c[20])(),c[18],c[35]),c[7])((0,c[69])(),c[18],c[62]),c[23]<=4&&(((0,c[5])(c[10],c[62]),c[5])(c[83],c[35]),(0,c[61])(c[17],c[35]),c[61])(c[2],c[3]),c[19]<=-10?((0,c[new Date("1970-01-01T01:45:37.000+01:45")/1E3])(c[35]),c[7])((0,c[29])(),c[18],c[62]):(0,c[22])((0,c[61])(c[8],c[62]),c[75],c[3],c[57])}catch(d){(0,c[31])((0,c[58])(c[64],c[29]),c[4],(0,c[4])(c[2]),c[41])}}catch(d){return"enhanced_except_3JwBo-P-_w8_"+
	   a}return b.join("")}`
	return evalJavascript(fBody, encoded)
}

func evalJavascript(jsFunction, arg string) (string, error) {
	const myName = "myFunction"

	vm := goja.New()
	_, err := vm.RunString(myName + "=" + jsFunction)
	if err != nil {
		return "", err
	}

	var output func(string) string
	err = vm.ExportTo(vm.Get(myName), &output)
	if err != nil {
		return "", err
	}

	return output(arg), nil
}

func (config playerConfig) getNFunction() (string, error) {
	nameResult := nFunctionNameRegexp.FindSubmatch(config)
	if len(nameResult) == 0 {
		return "", errors.New("unable to extract n-function name")
	}

	var name string
	if idx, _ := strconv.Atoi(string(nameResult[2])); idx == 0 {
		name = string(nameResult[4])
	} else {
		name = string(nameResult[1])
	}

	return config.extraFunction(name)

}

func (config playerConfig) extraFunction(name string) (string, error) {
	// find the beginning of the function
	def := []byte(name + "=function(")
	start := bytes.Index(config, def)
	if start < 1 {
		return "", fmt.Errorf("unable to extract n-function body: looking for '%s'", def)
	}

	// start after the first curly bracket
	pos := start + bytes.IndexByte(config[start:], '{') + 1

	var strChar byte

	// find the bracket closing the function
	for brackets := 1; brackets > 0; pos++ {
		b := config[pos]
		switch b {
		case '{':
			if strChar == 0 {
				brackets++
			}
		case '}':
			if strChar == 0 {
				brackets--
			}
		case '`', '"', '\'':
			if config[pos-1] == '\\' && config[pos-2] != '\\' {
				continue
			}
			if strChar == 0 {
				strChar = b
			} else if strChar == b {
				strChar = 0
			}
		}
	}

	return string(config[start:pos]), nil
}

func (config playerConfig) decrypt(cyphertext []byte) ([]byte, error) {
	operations, err := config.parseDecipherOps()
	if err != nil {
		return nil, err
	}

	// apply operations
	bs := []byte(cyphertext)
	for _, op := range operations {
		bs = op(bs)
	}

	return bs, nil
}

/*
parses decipher operations from https://youtube.com/s/player/4fbb4d5b/player_ias.vflset/en_US/base.js

var Mt={
splice:function(a,b){a.splice(0,b)},
reverse:function(a){a.reverse()},
EQ:function(a,b){var c=a[0];a[0]=a[b%a.length];a[b%a.length]=c}};

a=a.split("");
Mt.splice(a,3);
Mt.EQ(a,39);
Mt.splice(a,2);
Mt.EQ(a,1);
Mt.splice(a,1);
Mt.EQ(a,35);
Mt.EQ(a,51);
Mt.splice(a,2);
Mt.reverse(a,52);
return a.join("")
*/
func (config playerConfig) parseDecipherOps() (operations []DecipherOperation, err error) {
	objResult := actionsObjRegexp.FindSubmatch(config)
	funcResult := actionsFuncRegexp.FindSubmatch(config)
	if len(objResult) < 3 || len(funcResult) < 2 {
		return nil, fmt.Errorf("error parsing signature tokens (#obj=%d, #func=%d)", len(objResult), len(funcResult))
	}

	obj := objResult[1]
	objBody := objResult[2]
	funcBody := funcResult[1]

	var reverseKey, spliceKey, swapKey string

	if result := reverseRegexp.FindSubmatch(objBody); len(result) > 1 {
		reverseKey = string(result[1])
	}
	if result := spliceRegexp.FindSubmatch(objBody); len(result) > 1 {
		spliceKey = string(result[1])
	}
	if result := swapRegexp.FindSubmatch(objBody); len(result) > 1 {
		swapKey = string(result[1])
	}

	regex, err := regexp.Compile(fmt.Sprintf("(?:a=)?%s\\.(%s|%s|%s)\\(a,(\\d+)\\)", regexp.QuoteMeta(string(obj)), regexp.QuoteMeta(reverseKey), regexp.QuoteMeta(spliceKey), regexp.QuoteMeta(swapKey)))
	if err != nil {
		return nil, err
	}

	var ops []DecipherOperation
	for _, s := range regex.FindAllSubmatch(funcBody, -1) {
		switch string(s[1]) {
		case reverseKey:
			ops = append(ops, reverseFunc)
		case swapKey:
			arg, _ := strconv.Atoi(string(s[2]))
			ops = append(ops, newSwapFunc(arg))
		case spliceKey:
			arg, _ := strconv.Atoi(string(s[2]))
			ops = append(ops, newSpliceFunc(arg))
		}
	}
	return ops, nil
}
