export namespace main {

	export class AppInfo {
	    name: string;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class BibleSearchRequest {
	    translationId: string;
	    query: string;
	    mode: string;
	    limit: number;

	    static createFrom(source: any = {}) {
	        return new BibleSearchRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.translationId = source["translationId"];
	        this.query = source["query"];
	        this.mode = source["mode"];
	        this.limit = source["limit"];
	    }
	}
	export class BibleVerse {
	    bookId: number;
	    book: string;
	    chapter: number;
	    verse: number;
	    reference: string;
	    text: string;

	    static createFrom(source: any = {}) {
	        return new BibleVerse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bookId = source["bookId"];
	        this.book = source["book"];
	        this.chapter = source["chapter"];
	        this.verse = source["verse"];
	        this.reference = source["reference"];
	        this.text = source["text"];
	    }
	}
	export class BibleSearchResponse {
	    kind: string;
	    reference?: string;
	    results: BibleVerse[];

	    static createFrom(source: any = {}) {
	        return new BibleSearchResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.reference = source["reference"];
	        this.results = this.convertValues(source["results"], BibleVerse);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class TranslationView {
	    id: string;
	    name: string;
	    abbreviation: string;
	    language: string;
	    version: string;
	    verseCount: number;

	    static createFrom(source: any = {}) {
	        return new TranslationView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.abbreviation = source["abbreviation"];
	        this.language = source["language"];
	        this.version = source["version"];
	        this.verseCount = source["verseCount"];
	    }
	}

}
