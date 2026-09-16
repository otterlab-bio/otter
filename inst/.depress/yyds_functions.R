# check file or folder

checkMd5 <- function(masterFolder){
	cwd = getwd()
	setwd(masterFolder)
	files = listf("fr", masterFolder, ".md5$")
	for(file in files){
		setwd(dirname(file))
		system(paste("md5sum -c", file))
		setwd(cwd)
	}
}

waitJobEnd <- function(flag){
	while(TRUE){
		jobNames = system("qstat -r | grep 'Full jobname:' | awk '{print $3}'", intern = TRUE)
		if(!sum(grep(flag, jobNames))){
			break
		}else{
			Sys.sleep(10)
		}
	}
}

waitJob <- function(jobId){

	Sys.sleep(5)
	info = suppressWarnings(system(paste("qstat | grep", jobId), intern = TRUE))
	if(length(info) == 0){
		cat("Job end!\n")
	} else{
		state = subString(info, 2, "   ")
		if(state == "Eqw"){
			deleteEqw()
			cat("Job error!\n")
		} else{
			check = TRUE
			while(check){
				cat("Job is running, wait 30 secs\n")
				Sys.sleep(30)
				check = !length(suppressWarnings(system(paste("qstat | grep", jobId), intern = TRUE)))
				if(check){
					check = FALSE
				} else{
					check = TRUE
				}
			}
		}
	}
}

ensureExists <- function(folder){
	while(TRUE){
		if(dir.exists(folder))
			break
		else
			Sys.sleep(10)
	}
}

reportError  <- function(N, error){
	if(N){
		x = paste0(paste0("\"", error, "\""), sep = "", collapse = ", ")
		cat("Error folder(s):", x, "\n")
		cat(N, "folder(s) are error!\n")	
	} else {
		cat("All folders are normal!\n")
	}
}

reportRemove  <- function(N, error){
	if(N){
		x = paste0(paste0("\"", error, "\""), sep = "", collapse = ", ")
		cat("Error folder(s):", x, "\n")
		cat(N, "folder(s) are removed!\n")	
	} else {
		cat("All folders are normal!\n")
	}
}

checkErrorFolders  <- function(targetFolder, checkNumber){
	Sum = 0
	error = c()
	folders = system(paste("ls", targetFolder, "-F | grep '/$'"), intern = T)
	Ns = length(folders)
	for(i in 1:Ns){
		folder = folders[i]
		n = length(list.files(paste0(targetFolder, "/", folder)))
	
		if(n > checkNumber){
			cat("\tMore than", checkNumber, "file(s) in", basename(folder), "\n")
			Sum = Sum + 1
			error = c(error, basename(folder))
		} else if(n < checkNumber){
			cat("\tOnly", n, "file(s) in", basename(folder), "\n")
			Sum = Sum + 1
			error = c(error, basename(folder))
		}
	}
	reportError(Sum, error)
}

deleteEqw <- function(){

	jobNm = system("qstat | grep Eqw | awk '{print $3}'", intern = TRUE)
	jobID = system("qstat | grep Eqw | awk '{print $1}'", intern = TRUE)
	sapply(jobID, function(x) system(paste("qdel", x), ignore.stdout = TRUE))
	return(jobNm)
}

limitJobNumber <- function(Nlimit = 50, sleep = 10){

	deleteEqw()
	check = TRUE
	while(check){	
		USER  = system("echo $USER", intern = TRUE)
		N_job = system(paste("qstat | grep", USER, "| wc -l"), intern = TRUE)
		check = !as.numeric(N_job) < Nlimit
		if(check){
			cat("\t", Nlimit, "jobs are running, wait", sleep, "secs\n")
			deleteEqw()
			Sys.sleep(sleep)
		}
	}
}


checkJobsNumber  <- function(cycle, Nlimit = 40, sleep = 10, gap = 0.5){

	# deleteEqw()
	if((cycle %% 10) == 0)
	    Sys.sleep(gap)

	check = TRUE
	while(check){
		
		USER  = system("echo $USER", intern = TRUE)
		N_job = system(paste("qstat | grep", USER, "| wc -l"), intern = TRUE)
		check = !as.numeric(N_job) < Nlimit
		if(check){
			cat("\t", Nlimit, "jobs are running, wait", sleep, "secs\n")
			deleteEqw()
			Sys.sleep(sleep)
		}
	}
}

reportSGE <- function(cycle, cycles, tag, Nlimit, sleep){

	EqwJobs = c()
    cat("Job", i, tag, "submitted, Remian", cycles - cycle, "\n")
    checkJobsNumber(cycle = i, Nlimit = Nlimit, sleep = sleep)
    EqwJobs = c(EqwJobs, deleteEqw())
    return(EqwJobs)
}

getRunJobs <- function(){

	jobNm = system(paste("qstat | grep '   r     ' | awk '{print $3}'"), intern = TRUE)
	return(jobNm)
}

deleteJobs <- function(pattern = ""){

	jobNm = system(paste("qstat | grep", pattern,  "| awk '{print $3}'"), intern = TRUE)
	jobID = system(paste("qstat | grep", pattern,  "| awk '{print $1}'"), intern = TRUE)
	sapply(jobID, function(x) system(paste("qdel", x), ignore.stdout = TRUE))

}

removeErrorFolders  <- function(targetFolder, checkNumber){

	Sum = 0
	error = c()
	folders = system(paste("ls", targetFolder, "-F | grep '/$'"), intern = T)
	Ns = length(folders)
	for(i in 1:Ns){
		folder = folders[i]
		n = length(list.files(paste0(targetFolder, "/", folder)))
	
		if(n > checkNumber){
			cat("\tMore than", checkNumber, "file(s) in", basename(folder), "removed\n")
			unlink(paste0(targetFolder, "/", folder), recursive = TRUE)
			Sum = Sum + 1
			error = c(error, basename(folder))
		} else if(n < checkNumber){
			cat("\tOnly", n, "file(s) in", basename(folder), "removed\n")
			unlink(paste0(targetFolder, "/", folder), recursive = TRUE)
			Sum = Sum + 1
			error = c(error, basename(folder))
		}
	}
	reportRemove(Sum, error)
}

removeLogs  <- function(logFolder, tags){
	Sum = 0
	files = paste0(logFolder, "/", tags)
	files = paste0(files, rep(c(".out", ".err"), each = length(tags)))
	for(i in 1:length(files)){
		if(dir.exists(files[i])){
			unlink(files[i], recursive = TRUE)
			cat("File", basename(files[i]), "removed\n")
			Sum = Sum + 1
		} 
	}

	if(Sum == 0) {
			cat("All log files are non-existent or had been removed already!\n")
	}
}

checkIfExists  <- function(folders){
	for(folder in folders){
	    if(!dir.exists(folder))
	    	dir.create(folder, recursive = TRUE)
	}
}

findSrxMemoryExceed  <- function(logFolder){

	logErrs = list.files(logFolder, ".err", full.names = TRUE)
	Ns = length(logErrs)

	error = c()
	Sum = 0
	for(i in 1:Ns){
		logErr = logErrs[i]
		check = length(system(paste("cat", logErr, "| grep memory"), intern = TRUE))
		if(check != 0){
			tag = gsub(".err", "", basename(logErr))
			cat("Job", tag, "memory overflowed!\n")
			error = c(error, tag)
			Sum = Sum + 1
		}
	}
	reportErr(Sum, error)
}
# ColorPalettes for ploting

myColor <- function(){

	cat("Red -> #F93463\n")
	cat("Orange -> #F99F0C\n")
	cat("Yellow -> #F6EA22\n")
	cat("Green -> #39EF27\n")
	cat("Cyan -> #27EFC9\n")
	cat("Blue -> #27A2EF\n")
	cat("Purple -> #B127EF\n")
}


impute <- function(Tx, na.rm = 1, method = "mean", byGroup = NULL){	
	if(is.null(byGroup)){
		Tx1 = Tx[apply(Tx, 1, function(x) sum(is.na(x)) / ncol(Tx) < na.rm), ]
		Tx2 = apply(Tx1, 1, function(x) { x[is.na(x)] = mean(x, na.rm = TRUE); x })
	} else {
		bylist = rep(0, ncol(Tx))
		bylist = sapply(1:length(byGroup), function(x) { bylist[match(byGroup[[x]], colnames(Tx))] = x; bylist })
		bylist = list(apply(bylist, 1, sum))
		len = table(bylist[[1]])

		Tx1 = Tx[apply(Tx, 1, function(x) sum(aggregate(is.na(x), by = bylist, sum)[, 2] / len < na.rm) == length(len)), ]
		Tx2 = apply(Tx1, 1, function(x){ 
			for(i in 1:length(len)){
				x[is.na(x) & colnames(Tx1) %in% byGroup[[i]]] = mean(x[match(byGroup[[i]], colnames(Tx1))], na.rm = TRUE)
				return(x)
			}
			return(x)
		})
		Tx2 = t(Tx2)
	}
	return(Tx2)
}

gsub2 <- function(from, to, obj){
	for(i in 1:length(from)){
		if(length(to) == 1){
			obj = gsub(from[i], to, obj)
		}else{
			obj = gsub(from[i], to[i], obj)
		}
	}
	obj
}

reload <- function(){
	rm("MyEvaluation", envir = .GlobalEnv)
	source("~/.yyds")
}

strsplit2 <- function(x, sep, idx, psep = NULL){
	res = sapply(x, function(y) strsplit(y, sep)[[1]][idx], simplify = FALSE, USE.NAMES = FALSE)
	if(!is.null(psep)){
		sapply(res, function(z) paste(z, collapse = psep), simplify = TRUE, USE.NAMES = FALSE)	
	} else{
		as.character(res)
	}
}

rep2 <- function(x, each){
    if(length(each) == 1){
        return(rep(x, each = each))
    }else{
        res = c()
        for(i in 1:length(each)) res = c(res, rep(x[i], each = each[i]))
        return(res)
    }
}

lf <- function(type, folder, pattern = "", remove = NULL){
	if(!type %in% c("rf", "f", "r", "x"))
		stop("\nOnly support below types:
			\t- rf: recursive = TRUE, full.names = TRUE
			\t-  f: recursive = TRUE, full.names = FALSE
			\t-  r: recursive = FALSE, full.names = TRUE
			\t-  x: recursive = FALSE, full.names = FALSE\n")
	res = switch(type,
			rf = list.files(folder, pattern, full.names = TRUE, recursive = TRUE),
			f  = list.files(folder, pattern, full.names = TRUE, recursive = FALSE),
			r  = list.files(folder, pattern, full.names = FALSE, recursive = TRUE),
			x  = list.files(folder, pattern, full.names = FALSE, recursive = FALSE)
	)
	if(!is.null(remove))
		res = res[!grepl(remove, res)]
	res
}

subset2 <- function(x, condition) {
  condition_call <- substitute(condition)
  r <- eval(condition_call, x, parent.frame())
  x[r, ]
}

loadp <- function(...){
    pkgs = as.character(substitute(list(...)))[-1]
    suppressMessages(for(pkg in pkgs) require(pkg, character.only = TRUE))
}

rm2 <- function(){
	rm(list = ls(envir = .GlobalEnv), envir = .GlobalEnv)
}

write <- function(type, data, file, sep = "\t"){
	if(!type %in% c("rc", "r", "c", "x"))
		stop("\nOnly support below types:
			\t- rc: save rownames and colnames
			\t-  r: only save 
			\t-  c: only save colnames
			\t-  x: save neither rownames nor colnames\n")
	switch(type,
		rc = write.table(data, file, row.names = TRUE, col.names = TRUE, sep = "\t", quote = FALSE),
		r  = write.table(data, file, row.names = TRUE, col.names = FALSE, sep = "\t", quote = FALSE),
		c  = write.table(data, file, row.names = FALSE, col.names = TRUE, sep = "\t", quote = FALSE),
		x  = write.table(data, file, row.names = FALSE, col.names = FALSE, sep = "\t", quote = FALSE)
	)
}

read <- function(type, file, header = FALSE, sep = "\t"){
	if(!type %in% c("rc", "r", "c", "x"))
		stop("\nOnly support below types:
			\t- rc: save rownames and colnames
			\t-  r: only save 
			\t-  c: only save colnames
			\t-  x: save neither rownames nor colnames\n")
	switch(type,
		rc = read.table(data, file, header = TRUE, col.names = TRUE, sep = "\t", quote = FALSE),
		r  = read.table(data, file, header = TRUE, col.names = FALSE, sep = "\t", quote = FALSE),
		c  = read.table(data, file, header = FALSE, col.names = TRUE, sep = "\t", quote = FALSE),
		x  = read.table(data, file, header = FALSE, col.names = FALSE, sep = "\t", quote = FALSE)
	)
}

removeLetter <- function(string){

	idx = which(as.numeric(sapply(LETTERS[1:3], function(x) grep(x, string))) == 1)
	if(length(idx)){
		res = gsub(LETTERS[idx], "", string)
	} else{
		res = string
	}
	return(res)
}

# <--------- GenomicRanges --------->
mergeGrStrand <- function(gr){

	Tx = data.frame(gr)
	CGI = paste0(seqnames(gr), ":", start(gr), "-", end(gr))
	mTx = aggregate(mcols(gr)[, 1:2], by = list(factor(CGI, levels = unique(CGI))), sum)[, -1]
	mgr = toGR(unique(CGI))
	beta = signif(mTx[, 1] / mTx[, 2], 4)
	mcols(mgr) = data.frame(mTx, beta, SID = unique(gr$SID))
	return(mgr)
}

buildGR <- function(Chr, ST, ED, strand = "*", meta = NA){

	suppressMessages(library("GenomicRanges"))
	Chr = as.character(Chr)
	ST = as.numeric(ST)
	ED = as.numeric(ED)
	gr = GRanges(seqnames = Rle(Chr),  ranges = IRanges(ST, ED), strand = strand)
	if(class(meta) == "data.frame"){
		mcols(gr) = meta	
	} else if(class(meta) != "logical"){
		stop("'meta' must be a dataframe.")
	}
	return(gr)
}

grListToGR <- function(grList){

	return(unlist(GRangesList(grList)))
}

getOverlaps <- function(gr, Intervals){

	x = countOverlaps(gr, Intervals, type = "any", ignore.strand = TRUE)
	if(!length(x)){	
		stop("No common seqname or range in the two GRanges objects.")
	}
	res = gr[x == 1]
	return(res)
}

aggregateOverlaps  <- function(gr, Intervals, ignore.strand = TRUE){

	suppressMessages(library("GenomicRanges"))
	gr = gr[countOverlaps(gr, Intervals, type = "any", ignore.strand = ignore.strand) == 1]
	x  = findOverlaps(gr, Intervals, type = "any", ignore.strand = ignore.strand)
	cM = data.frame(mcols(gr))
	if(nrow(cM) == 0){
		stop("No common seqname or range in the two GRanges objects.")
	} else{
		agT = aggregate(cM[, 1:2], by = list(subjectHits(x)), sum)
		mgr = Intervals[as.integer(agT[,1])]
		mcols(mgr) = agT[,2:3]
		mgr$beta = signif(agT[, 2] / agT[, 3], 4)
		if(exists("tag")) mgr$SID = tag
	}
	return(mgr)
}

aggregateOverlaps2  <- function(gr, Intervals){

	suppressMessages(library("GenomicRanges"))
	gr = gr[countOverlaps(gr, Intervals) == 1]
	x  = findOverlaps(gr, Intervals)
	cM = data.frame(mcols(gr))
	if(nrow(cM) == 0){
		stop("No common seqname or range in the two GRanges objects.")
	} else{
		agT = aggregate(cM[, 1:2], by = list(subjectHits(x)), sum)
		mgr = Intervals[as.integer(agT[,1])]
		mcols(mgr) = agT[,2:3]
		mgr$beta = signif(agT[, 2] / agT[, 3], 4)
		if(exists("tag")) mgr$SID = tag
	}
	return(mgr)
}

toGR  <- function(obj, strandIdx = NA){

	suppressMessages(library("GenomicRanges"))
	type = class(obj)[1]
	if(type == "GRanges"){
		GR = obj
	} else if(type == "list"){
		GR = unlist(GRangesList(grList))
	} else if(type == "character"){

		chr = sapply(strsplit(obj, "[-:]"), function(z) z[1])
		ST = as.numeric(sapply(strsplit(obj, "[-:]"), function(z) z[2]))
		ED = as.numeric(sapply(strsplit(obj, "[-:]"), function(z) z[3]))
		SD = as.character(sapply(strsplit(obj, "[-:]"), function(z) z[4]))
		GR = buildGR(chr, ST, ED, SD)
	} else if(type == "data.frame"){

		chr = obj[, 1]
		ST  = obj[, 2]
		ED  = obj[, 3]

		if(ncol(obj) > 3){
			if(obj[1, 4] %in% c("+", "-", "*")){
				if(ncol(obj) > 4){

					meta = obj[, 5:ncol(obj)]
					strand = obj[, 4]
					GR = buildGR(chr, ST, ED, strand, meta)  
				} else{
					strand = obj[, 4]
					GR = buildGR(chr, ST, ED, strand)  
				}
			} else if(!is.na(strandIdx)){

				strand = obj[, strandIdx]
				meta = obj[, -c(1:3, strandIdx)]
				GR = buildGR(chr, ST, ED, strand, meta)  
		    } else{
				meta = obj[, -c(1:3), drop = FALSE]
				GR = buildGR(chr, ST, ED, meta = meta)  
		    }
		} else{
			GR = buildGR(chr, ST, ED)
		}
	}
	return(GR)
}

bedGraphToGR <- function(Tx){

	chr = as.character(Tx[,1])
	ST = as.numeric(Tx[,2]) + 1
	ED = as.numeric(Tx[,3])
	methyCount = as.numeric(Tx[,5])
	readCount = methyCount + as.numeric(Tx[,6])
	beta = signif(methyCount / readCount, 4)
	gr = GRanges(seqnames = Rle(chr),  ranges = IRanges(ST,ED),  strand = "*", methyCount, readCount, beta)
	return(gr)
}

grToBed  <- function(GR){

	suppressMessages(library("GenomicRanges"))
	seqs   = as.character(GR)
	chr    = sapply(strsplit(seqs, "[-:]"), function(z) z[1])
	ST     = start(GR)
	ED     = end(GR)
	Strand = strand(GR)
	metas  = mcols(GR)
	bed    = data.frame(chr, ST, ED, Strand, metas)	
	return(bed)
}

grToFasta <- function(gr, file){

	hg19.fa = "/sibcb2/bioinformatics2/wangjiahao/iGenome/Bismark/hg19/hg19.fa"
	pos = as.character(gr)
	write.table(pos, "tmp.pos", sep = "\n", quote = FALSE, col.names = FALSE, row.names = FALSE)
	system(paste("samtools faidx -n 50", hg19.fa, "-r tmp.pos >", file))
	system(paste("rm tmp.pos & samtools faidx", file))
}

# <--------- string manipulation --------->
subString <- function(strings, idx, sep = NA){

	strings = as.character(strings)
	if(is.na(sep)){
		res = as.character(lapply(strings, function(x) paste(strsplit(x, "")[[1]][idx], collapse = "")))
	} else{
		res = sapply(strsplit(strings, sep), function(x) x[idx])
	}
	return(res)
}

revString <- function(strings){

	strings = as.character(strings)
	res = as.character(lapply(strings, function(x) paste(rev(strsplit(x, "")[[1]]), collapse = "")))
	return(res)
}

revCompString <- function(strings){

	strings = toupper(strings)	
	idx = data.frame(to = c("T", "C", "G", "A", "N"), row.names = c("A", "G", "C", "T", "N"))
	res = as.character(sapply(strings, function(x) revString(paste(as.character(idx[strsplit(x, "")[[1]], "to"]), collapse = ""))))
	# chars = strsplit(strings, "")[[1]]
	# res = revString(paste(as.character(idx[chars, "to"]), collapse = ""))
	return(res)
}

subsplit <- function(string, N = 2){

	string = as.character(string)
	seqs = c()
	for(i in 1:(nchar(string) - N + 1)){
		seqs = c(seqs, subString(string, i:(i + N - 1)))
	}
	return(seqs)
}

matchPattern2 <- function(string, pattern, ignore.case = FALSE){

	string = as.character(string)
	if(ignore.case){
		string = tolower(string)
		pattern = tolower(pattern)		
	}
	seqs = subsplit(string, nchar(pattern))
	start = which(seqs == pattern)
	end = start + nchar(pattern) - 1
	res = data.frame(start, end)
	return(res)
}

countPattern <- function(string, pattern, ignore.case = FALSE){

	res = nrow(matchPattern2(string, pattern, ignore.case = ignore.case))
	return(res)
}

grepr <- function(string, pattern, ignore.case = FALSE){

	res = matchPattern2(string, pattern, ignore.case = ignore.case)[, 1]
	return(res)
}

greprl <- function(string, pattern, ignore.case = FALSE){

	res = rep(FALSE, nchar(string))
	idx = matchPattern2(string, pattern, ignore.case = ignore.case)[, 1]
	res[idx] = TRUE
	return(res)
}

paster <- function(strs1, strs2, sortBy = 2, start = "", end = "", sep = ""){

    res1 = unlist(lapply(strs1, function(x) paste0(x, sep, strs2)))
    res2 = paste0(start, res1, end)
    # if(sortBy == 2){
	   #  res2 = as.character(sapply(strs2, function(x) grep(x, res2, value = TRUE)))
    # }
    return(res2)
}

toTitle <- function(string){

	Title = toupper(subString(string, 1))
	res = paste0(Title, subString(tolower(string), 2:nchar(string)))
    return(res)
}

countNA <- function(Matrix){

	x = table(is.na(Matrix))
	return(c(ALL = sum(x), x))
}

# <--------- table manipulation --------->
write.line <- function(x, file){
	write.table(x, file, col.names = FALSE, row.names = FALSE, quote = FALSE, sep = "\t")
}

hd <- function(obj, x = 5, y = NULL){

	if(class(obj) == "function")
		return(head(obj))

	check_len <- function(idx, max){
		if(length(idx) > 1){
			if(max(idx) > max){
				idx_res = idx[idx <= max]
			}else{
				idx_res = idx
			}
		}else{
			if(idx > max){
				idx_res = 1:max
			}else{
				idx_res = 1:idx
			}
		}
		return(idx_res)
	}

	if(is.null(y))
		y = x
	
	dims = is.null(dim(obj))
	if(!dims){

		cat("dim:", nrow(obj), "×", ncol(obj), "\n")

		idx_x = check_len(x, nrow(obj))
		idx_y = check_len(y, ncol(obj))
		res = data.frame(obj[idx_x, idx_y])
		if(!is.null(rownames(obj)))
			rownames(res) = rownames(obj)[idx_x]
		if(!is.null(colnames(obj)))
			colnames(res) = colnames(obj)[idx_y]
	} else{
		cat("dim:", length(obj), "\n")
		idx_x = check_len(x, length(obj))
		res = obj[idx_x]
		if(!is.null(names(obj)))
			names(res) = names(obj)[idx_x]
	}
	return(res)
}

sortBy <- function(Tx, by, decreasing = FALSE){

	Tx = Tx[order(Tx[, by[1]], decreasing = decreasing), ]
	if(length(by) > 1){
		UNI_Name = unique(Tx[, by[1]])
		for(i in 1:length(UNI_Name)){
			xTx = Tx[Tx[, by[1]] == UNI_Name[i], ]
			xTx_sorted = xTx[order(xTx[, by[2]], decreasing = decreasing), ]
			if(i == 1){
				res = xTx_sorted
			} else{
				res = rbind(res, xTx_sorted)
			}
		}
	}
	return(res)
}

removeRowsAllNa  <- function(x) x[apply(x, 1, function(y) any(!is.na(y))),]

removeColsAllNa  <- function(x) x[, apply(x, 2, function(y) any(!is.na(y)))]

fillNA <- function(Matrix, by = 2, method = "median"){

	replaceNA <- function(x, method){
		if(method == "median"){
			x[is.na(x)] = median(x, na.rm = TRUE) 
		}else{
			x[is.na(x)] = mean(x, na.rm = TRUE) 
		}
		return(x)
	}

	if(by == 2){
		res = apply(Matrix, 2, function(x) replaceNA(x, method)) 
	} else{
		res = apply(Matrix, 1, function(x) replaceNA(x, method)) 
	}

	return(as.data.frame(res))
}

colsAsFactor <- function(Matrix, omits = c("NA", "*")){
    
    NewM = Matrix
    for(i in 1:ncol(Matrix)){
        
        x = as.character(Matrix[, i])
        x[x %in% omits] = NA
        check = !is.na(suppressWarnings(as.numeric(na.omit(x)[1])))
        if(check)
            next
        Levels = sort(na.omit(unique(x)))
        Nums = sapply(x, function(y) (1:length(Levels))[match(y, Levels)])
        NewM[, i] = factor(Nums)
    }
    return(NewM)
}

removeCols <- function(Matrix, cutoff = 0.85){
	
	removeID = c()
	for(i in 1:ncol(Matrix)){

		tag = colnames(Matrix)[i]
		x = Matrix[, i]
		ratio = sum(!is.na(x)) / length(x)
		if(ratio < cutoff) 
			removeID = c(removeID, tag)
	}
	res = Matrix[, -match(removeID, colnames(Matrix))]

	# report result
	N = length(removeID)
	Ratio = paste0(signif(N / ncol(Matrix), 3) * 100, "%") 
	cat("\t", paste0(N, "(", Ratio, ") variables removed.\n"))
	return(res)
}

checkSamplesCoverage <- function(Matrix, ratio){

	idx = apply(Matrix, 1, function(x) length(which(is.na(x))) / length(x) >= ratio*0.01)
	return(Matrix[idx,])
}

# <--------- job control --------->
ensureJobEnd <- function(Job = "Job"){

	check = TRUE
	while(check){
		check = suppressWarnings(length(system(paste("qstat | grep", Job), intern = TRUE)))
		if(check) Sys.sleep(1)
	}
}

ResubmitJobs  <- function(){
	check = TRUE
	while(check){
		submitJobs()
		Sys.sleep(5)
		SRX = list.files(masterFolder_source)
		done = list.files(masterFolder_target)
		do = setdiff(SRX, done)
		if(length(do) == 0){
			cat("\tComplete!\n")
			check = FALSE
		} else{
			cat("\tResubmit", length(do), "jobs\n")
			check = TRUE
		}
	}
}

# <--------- read data --------->
read.faster <- function(file, header = FALSE, sep = "\t", showProgress = TRUE, skip = 0, nrows = -1){

	suppressPackageStartupMessages(library("data.table"))
	Tx = data.frame(fread(file = file, header = header, sep = sep, showProgress = showProgress, skip = skip, nrows = nrows))
	return(Tx)
}

read.bedgraph <- function(file, showProgress = FALSE, nrows = -1){

	Tx = read.faster(file = file, header = FALSE, sep = "\t", skip = 1, showProgress = showProgress, nrows = nrows)
	return(Tx)
}

read.gr <- function(file, metaNames = NULL){

	format = tail(strsplit(file, "\\.")[[1]], 1)
	if(format == "bed"){
		res = toGR(read.faster(file, sep = "\t", showProgress = FALSE))
	} else if(format == "RData"){
		res = get(load(file))
	} else if(format == "bedGraph"){
		res = bedGraphToGR(read.bedgraph(file))
	}
	if(!is.null(metaNames))
		colnames(mcols(res)) = metaNames
	return(res)
}

read.gmt <- function (file){

    if (!grepl("\\.gmt$", file)[1]) {
        stop("Pathway information must be a .gmt file")
    }
    geneSetDB = readLines(file)
    geneSetDB = strsplit(geneSetDB, "\t")
    names(geneSetDB) = sapply(geneSetDB, "[", 1)
    geneSetDB = lapply(geneSetDB, "[", -1:-2)
    geneSetDB = lapply(geneSetDB, function(x) {
        x[which(x != "")]
    })
    return(geneSetDB)
}

getNLine <- function(file){
    
	res = as.numeric(subString(system(paste("wc -l", file), intern = TRUE), 1, " "))
    return(res)
}

read.fasta <- function(filePath){

	N = as.numeric(subString(system(paste("cat", filePath, "| grep '>' -n"), intern = TRUE), 1, ":"))
	lines = readLines(filePath)
	seqs = c()
	for(i in 1:length(N)){

		if(i != length(N)){
			xlines = readLines(filePath)[(N[i] + 1):(N[i + 1] - 1)]
		} else{
			Nlines = getNLines(file)
			xlines = readLines(filePath)[(N[i] + 1):Nlines]
		}
		seqs = c(seqs, paste(xlines, collapse = ""))
	}

	Pos = subString(lines[N], 2, ">")  
	names(seqs) = Pos
	res = as.list(seqs)
	return(res)
}

# getSeq <- function(POSs){
	
# 	grToFasta(toGR(POSs), file = "tmp.fa")
# 	res = read.fasta("tmp.fa")[[1]]
# 	system("rm tmp.fa tmp.fa.fai")
# 	return(res)
# }

getSeq <- function(POSs){
	
	genomeFile = "/sibcb2/bioinformatics2/wangjiahao/iGenome/Bismark/hg19/hg19.fa"
	res = system(paste("samtools faidx", genomeFile, POSs), intern = TRUE)[2]
	return(res)
}

countProp <- function(string, patterns, ignore.case = FALSE){

	if(ignore.case){
		string  = tolower(string)
		pattern = tolower(pattern)
	}
	res = countPattern(string, patterns, ignore.case = FALSE) / nchar(string)
	return(res)
}


getReadsNumber <- function(file){
	Reads = as.numeric(strsplit(readLines(file)[1], "[ \t]")[[1]][6])
	return(Reads)
}

workflowToGR <- function(masterFolder, tag){

    Rscript = "/sibcb2/bioinformatics2/wangjiahao/software/Miniconda3/bin/Rscript"
    R_script = "/sibcb2/bioinformatics2/wangjiahao/code/Run/Merge_to_GR.R"
    cmd = paste(Rscript, R_script, "--runFolder", masterFolder, "--tag", tag) 
    suppressWarnings(system(cmd))
}

printSeq <- function(Seq, binwidth = 50){
	N_line = nchar(Seq) / binwidth
	if(N_line > as.integer(N_line)){
		N =  as.integer(N_line) + 1
	} else{
		N =  as.integer(N_line)
	}

	bases = strsplit(Seq, "")[[1]]
	for(i in 0:(N-1)){
		xseq = paste0(bases[(1 + binwidth*i):(binwidth + i*binwidth)], collapse = "") 
		if(i != (N-1)){
			xseq = paste0(bases[(1 + binwidth*i):(binwidth + i*binwidth)], collapse = "") 
		} else{
			xseq = paste0(bases[(1 + binwidth*i):length(bases)], collapse = "") 
		}
		cat(xseq, "\n")
	}
}

mean0 = function(x) mean(x, na.rm = TRUE)

getSize <- function(file, unit = "m"){

	if(unit == "k"){
		size = as.numeric(strsplit(system(paste("du -k", file), intern = TRUE), "\t")[[1]][1])
	} else{
		size = as.numeric(strsplit(system(paste("du -m", file), intern = TRUE), "\t")[[1]][1])
	}
	return(size)
}

# functions for data downstream analysis

mean0 = function(z) mean(z, na.rm = TRUE) # by boss

arrayIntervalMethylation <- function(MeM, IntervalGR){ # by boss

	source("/public1/home/scg5712/SHARE3/code/yyds/Methylation_workflow.R")
	if(!exists("RnBeads_450K_hg19_GR"))
		load("/public1/home/scg5712/SHARE3/iGenome/hg19/RData/RnBeads_450K_hg19_Probes_GR.RData") # RnBeads_450K_hg19_GR
	cpgGR = RnBeads_450K_hg19_GR

	# check seqlevels
	if(length(intersect(seqlevels(IntervalGR), seqlevels(cpgGR))) == 0)
		stop("No shared seqlevels found.\n")

	# only keep CpGs that locate in intervals
	Idx1 = countOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE) > 0
	Idx2 = names(cpgGR) %in% rownames(MeM)
	Idx  = Idx1 & Idx2

	if(sum(Idx) == 0)
		stop("No CpGs found in pre-defined regions.\n")
	
	cpgGR = cpgGR[Idx, ]

	# keep shared CpGs
	cNames = intersect(rownames(MeM), names(cpgGR))
	cpgGR  = cpgGR[cNames]
	MeM    = MeM[cNames, ]
	
	# Sample-by-sample processing to control memory footprint
	mergedM = matrix(NA, length(IntervalGR), ncol(MeM))
	x = findOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE)
	aT  = aggregate(MeM, by = list(factor(subjectHits(x))), FUN="mean0")
	aID = as.character(aT[,1])
	aMM = as.matrix(aT[,-1])
	mergedM[as.integer(aID), ] = aMM
	dimnames(mergedM) = list(as.character(IntervalGR), colnames(MeM))
	
	## return
	return(mergedM)

}

convertEnrichT <- function(type, enrichT){
	Tx = data.frame(enrichT)
	if(type == "go"){
		Tx$geneID = as.character(unlist(sapply(Tx$geneID, function(x) paste0(unique(strsplit(x, "/")[[1]]), collapse = "/"))))
	} else{
		Tx$Symbol = as.character(unlist(sapply(Tx$geneID, function(x) paste0(unique(sort(convertId(strsplit(x, "/")[[1]], "ENTREZID", "SYMBOL"))), collapse = "/"))))
	}
	Tx
}



## pathway enrichment
runEGO <- function(sigG, 
				   pvalueCutoff = 0.05, 
				   qvalueCutoff = 0.05,
				   universe = NULL, 
				   out = NULL,
				   keyType = "ENSEMBL" ,
				   ont = "ALL"
				   ){

	cat("Loading packages ..\n")
	suppressPackageStartupMessages(library("clusterProfiler"))
	suppressPackageStartupMessages(library("org.Hs.eg.db"))
	suppressPackageStartupMessages(library("msigdbr"))

	cat("Running", ont, "...\n")
	if(is.null(universe)){
		suppressMessages(
		ego <- enrichGO(
			gene          = sigG,
			keyType 	  = keyType,
			OrgDb         = org.Hs.eg.db,
			ont           = ont,
			pAdjustMethod = "fdr",
			pvalueCutoff  = pvalueCutoff,
			qvalueCutoff  = qvalueCutoff,
			readable      = TRUE
		))
	}else{
		suppressMessages(
		ego <- enrichGO(
			gene          = sigG,
			keyType 	  = keyType,
			OrgDb         = org.Hs.eg.db,
			ont           = ont,
			universe      = universe,
			pAdjustMethod = "fdr",
			pvalueCutoff  = pvalueCutoff,
			qvalueCutoff  = qvalueCutoff,
			readable      = TRUE
		))
	}

	egoT = convertEnrichT("go", data.frame(ego))
	if(!is.null(out))
		write.table(egoT, file = out, row.names = FALSE, col.names = TRUE, sep = "\t", quote = FALSE)

	if(is.null(nrow(egoT))){
		cat("No terms enriched\n")
		egoT[1, ] = rep(NA, ncol(egoT))
	} else{
		cat(paste(nrow(egoT), "terms enriched\n"))
	}
	return(egoT)
}

runKEGG <- function(sigG, 
				    pvalueCutoff = 0.05, 
				    qvalueCutoff = 0.05,
				    organism = "hsa",
					out = NULL
					){

	suppressPackageStartupMessages(library("clusterProfiler"))
	suppressPackageStartupMessages(library("org.Hs.eg.db"))

	suppressMessages(
	ekegg <- enrichKEGG(
		gene          = sigG,
		organism      = organism,
		keyType 	  = "kegg",
		pAdjustMethod = "fdr", 
		pvalueCutoff  = pvalueCutoff,
		qvalueCutoff  = qvalueCutoff,
		use_internal_data = FALSE
	))

	ekeggT = convertEnrichT("kegg", ekegg)
	if(!is.null(out))
		write.table(ekeggT, file = out, row.names = FALSE, col.names = TRUE, sep = "\t", quote = FALSE)

	if(is.null(nrow(ekeggT))){
		cat("No terms enriched\n")
		ekeggT[1, ] = rep(NA, ncol(ekeggT))
	} else{
		cat(paste(nrow(ekeggT), "terms enriched\n"))
	}
	return(ekeggT)
}

keggtToGeneT <- function(ekegg){

	geneList = strsplit(ekegg$Symbol, "/")
	genes = sort(unique(unlist(geneList)))
	resT = data.frame()
	for(Gene in genes){

		Pathway = paste(ekegg$Description[sapply(geneList, function(x) Gene %in% x)], collapse = " / ") 
		xresT = data.frame(Gene, Pathway)
		resT  = rbind(resT, xresT) 
	}
	return(resT)
}

write.gmt <- function(gsList, gmt_file){

	sink(gmt_file)
	for (i in 1:length(gsList)){

		cat(names(gsList)[i])
		cat('\tNA\t')
		cat(paste(gsList[[i]], collapse = '\t'))
		cat('\n')
	}
	sink()
}

runGSEA <- function(DEGtable  = NULL, 
					geneSet   = NULL, 
					rnkFile   = NULL, 
					chipFile  = NULL, 
					gmtFile   = NULL,
					prefix    = "GSEA",
					outFolder = "GSEA",
					min 	  = 15,
					max		  = 500, 
					verbo     = FALSE,
					PDF       = FALSE
					){

	if(prefix %in% list.files(outFolder))
		stop("\tPrefix already exists! please replace prefix.")

	if((is.null(geneSet) & is.null(gmtFile)) | (!is.null(geneSet) & !is.null(gmtFile)))
		stop("\tParameter 'geneSet' or 'gmtFile' must and only be supplied one.")

	check = suppressMessages(suppressWarnings(require("xlsx")))
	if(!check)
		warning("\tPackage 'xlsx' is not exist, results will save as txt format instead.")

	tmpFolder = paste0(outFolder, "/tmp_", prefix)
	dir.create(tmpFolder)

	if(!is.null(DEGtable)){

		cat(" Prepare files ...\n")
		Tx = read.table(DEGtable, row.names = 1, header = TRUE, sep = "\t")
		Tx <- Tx[!is.na(Tx$pvalue), ]
		Tx <- Tx[!is.na(Tx$log2FoldChange), ]

		ID <- rownames(Tx)
		ENSG <- sapply(strsplit(ID, " "), function(z) z[1])
		Symbol <- sapply(strsplit(ID, " "), function(z) z[2])
		pvalue <- as.numeric(Tx$pvalue)
		pvalue[pvalue < 10^-300] <- 10^-300
		zscore <- qnorm(pvalue/2, mean = 0, sd = 1, lower.tail = FALSE, log.p = FALSE)*sign(Tx$log2FoldChange)
		oTable <- data.frame(ENSG, zscore)
		rnkFile <- paste0(tmpFolder, "/", prefix, ".rnk")
		write.table(oTable, file = rnkFile, row.names = FALSE, col.names = FALSE, sep = "\t", quote = FALSE)

		chip = na.omit(data.frame(ENSG, Symbol, Symbol))
		colnames(chip) = c("Probe Set ID", "Gene Symbol", "Gene Title")
		chip = chip[!duplicated(as.character(chip$"Probe Set ID")), ]
		chipFile = paste0(tmpFolder, "/", prefix, ".chip")
		write.table(chip, file = chipFile, row.names = FALSE, col.names = TRUE, sep = "\t", quote = FALSE)

		if(!is.null(geneSet)){
			gmtFile = paste0(tmpFolder, "/", prefix, ".gmt")
			write.gmt(geneSet, gmtFile)
		}
	}

	# run GSEA
	cat(" Run GSEA ...\n")
	cmd = "java -cp /sibcb2/bioinformatics/software/GSEA/gsea2-2.2.4.jar xtools.gsea.GseaPreranked"
	cmd = paste(cmd, "-gmx", gmtFile)
	cmd = paste(cmd, "-rnk", rnkFile)
	cmd = paste(cmd, "-chip", chipFile)
	cmd = paste(cmd, "-rpt_label", prefix)
	cmd = paste(cmd, "-out", outFolder)
	cmd = paste(cmd, "-scoring_scheme weighted -collapse true -mode Max_probe -norm None")
	cmd = paste(cmd, "-nperm 1000 -include_only_symbols true")
	cmd = paste(cmd, "-make_sets true -plot_top_x 20 -rnd_seed timestamp")
	cmd = paste(cmd, "-set_min", min, "-set_max", max, "-zip_report false -gui false")
	if(verbo){
		system(cmd)
	}else{
		system(cmd, ignore.stdout = TRUE, ignore.stderr = TRUE)
	} 
	system(paste("rm -r", tmpFolder))

	# get GSEA plots in PDF format
	if(PDF){
		cat(" Get PDF format ...\n")
		gseaRunFolder = list.files(outFolder, paste0(prefix, ".GseaPreranked"), full.names = TRUE)
		if(length(gseaRunFolder) != 1)
			stop("None or multiple GSEA run was found.\n")
		cmd = paste("/sibcb2/bioinformatics/software/BcbioNG/anaconda/bin/gseapy replot -i", gseaRunFolder, "-o", gseaRunFolder)
		system(cmd, ignore.stdout = TRUE, ignore.stderr = TRUE)
	}

	# summary
	gseaRunFolder = list.files(outFolder, paste0(prefix, ".GseaPreranked"), full.names = TRUE)
	newRunFolder  = gsub(basename(gseaRunFolder), prefix, gseaRunFolder)
	system(paste("mv", gseaRunFolder, newRunFolder))

	neg = grep(".xls", list.files(newRunFolder, "gsea_report_for_na_neg", full.names = TRUE), value = TRUE)
	pos = grep(".xls", list.files(newRunFolder, "gsea_report_for_na_pos", full.names = TRUE), value = TRUE)

	posT = read.table(pos, header = TRUE, sep = "\t")[, 1:11]
	negT = read.table(neg, header = TRUE, sep = "\t")[, 1:11]
	resT = rbind(posT, negT)[, c(1, 4, 5, 7, 8)]
	resT$State = c(rep("UP", nrow(posT)), rep("DN", nrow(negT)))

	resT = resT[order(resT$FDR), ]
	dimnames(resT) = list(1:nrow(resT), c("Pathyway", "Size", "NES", "Pvalue", "FDR", "State"))
	resT[, 3:5] = apply(resT[, 3:5], 2, function(x) signif(x, 3))

	if(check){
		resL = split(resT, factor(resT$State, levels = c("UP", "DN")))
		if(!nrow(resL[["UP"]]))
			resL[["UP"]] = matrix(NA, 1, 6, dimnames = list(1, colnames(resT)))
		if(!nrow(resL[["DN"]]))
			resL[["DN"]] = matrix(NA, 1, 6, dimnames = list(1, colnames(resT)))

		write.xlsx(resL[["UP"]], file = paste0(newRunFolder, "/00_", prefix, "_GSEA_summary.xlsx"), sheetName = "UP", row.names = FALSE, append = TRUE)
		write.xlsx(resL[["DN"]], file = paste0(newRunFolder, "/00_", prefix, "_GSEA_summary.xlsx"), sheetName = "DN", row.names = FALSE, append = TRUE)
	} else{
		write.table(resT, file = paste0(newRunFolder, "/00_", prefix, ".txt"), sep = "\t", quote = FALSE, row.names = FALSE)
	}
	invisible(resL)
}

summaryGSEA <- function(gseaRunFolder){

	resL = list()
	names = c("pos", "neg")
	for(name in names){
		reportFile = list.files(gseaRunFolder, paste0("gsea_report_for_na_", name), full.name = TRUE)
		reportFile = reportFile[grep("xls$", reportFile)]
		reportTx = read.table(reportFile, header = TRUE, sep = "\t")

		pathways = as.character(reportTx$NAME)[reportTx$GS.DETAILS != ""]
		coreEnricheds = c()
		for(pathway in pathways){

			file  = paste0(gseaRunFolder, "/", pathway, ".xls")
			xresT = read.table(file, header = TRUE, sep = "\t")
			coreEnriched = as.character(xresT$PROBE[xresT$CORE.ENRICHMENT == "Yes"])
			coreEnriched = paste(coreEnriched, collapse = "/")
			coreEnricheds = c(coreEnricheds, coreEnriched)
		}
		outTx = reportTx[reportTx$GS.DETAILS != "", c(1, 4, 6, 7, 8)]
		outTx$coreEnrichment = coreEnricheds
		colnames(outTx) = c("pathway", "size", "NES", "pvalue", "FDR", "coreEnrichment")
		resL[[name]] = outTx
	}
	return(resL)
}

getKEGG <- function(ID){

	library("KEGGREST")

	gsList = list()
	for(xID in ID){

		gsInfo = keggGet(xID)[[1]]
		if(!is.null(gsInfo$GENE)){
			geneSetRaw = sapply(strsplit(gsInfo$GENE, ";"), function(x) x[1])	
			xgeneSet = list(geneSetRaw[seq(2, length(geneSetRaw), 2)])			
			NAME = sapply(strsplit(gsInfo$NAME, " - "), function(x) x[1])
			names(xgeneSet) = NAME
			gsList[NAME] = xgeneSet 
		} else{
			cat(" ", xID, "No corresponding gene set in specific database.\n")
		}
	}
	return(gsList)
}

findGO <- function(pattern, method = "key"){

	if(!exists("GO_DATA"))
		load("/sibcb2/bioinformatics2/wangjiahao/Data/RData/GO_DATA.RData")

	if(method == "key"){
		pathways = cbind(GO_DATA$PATHID2NAME[grep(pattern, GO_DATA$PATHID2NAME)])
	} else if(method == "gene"){
		pathways = cbind(GO_DATA$PATHID2NAME[GO_DATA$EXTID2PATHID[[pattern]]])
	}

	colnames(pathways) = "pathway"

	if(length(pathways) == 0){
		cat("No results!\n")
	} else{
		return(pathways)
	}
}

getGO <- function(ID){

	if(!exists("GO_DATA"))
		load("/sibcb2/bioinformatics2/wangjiahao/Data/RData/GO_DATA.RData")

	allNAME = names(GO_DATA$PATHID2EXTID)
	if(ID %in% allNAME){
		geneSet = GO_DATA$PATHID2EXTID[ID]
		names(geneSet) = GO_DATA$PATHID2NAME[ID]
		return(geneSet)		
	} else{
		cat("No results!\n")
	}
}

dotplot2 <- function(data, 
					 w 		   = 6, 
					 h 		   = 6, 
					 n 		   = 20, 
					 colorHigh = "#DD1C77", 
					 colorLow  = "#3182bd",
					 sortBy    = "p.adjust", 
					 title     = "", 
					 padjust   = TRUE,
					 plot      = TRUE,
					 out 	   = NULL
					 ){

	if(plot & is.null(out))
		stop("Please specific parameter 'out' to save pdf file.\n")

	if(n > nrow(data))
		n = nrow(data)
	data = data.frame(data)[1:n, ]

	data = data[order(data$Count, decreasing = TRUE), ]
	if(sortBy == "Count"){
		data$Description = factor(data$Description, levels = rev(data$Description))
	}else if(sortBy == "p.adjust"){
		if(padjust){
			data = data[order(data$p.adjust), ]
			data$Description = factor(data$Description, levels = rev(data$Description))
		}else{
			data = data[order(data$pvalue), ]
			data$Description = factor(data$Description, levels = rev(data$Description))
		}			
	}

	data$GeneRatioValue = as.numeric(subString(data$GeneRatio, 1, "/")) / as.numeric(subString(data$GeneRatio, 2, "/"))
	data = data[order(data$GeneRatioValue, decreasing = TRUE), ]

	p <- ggplot(data, aes(x = GeneRatioValue, y = Description)) + theme_bw()
	if(padjust){
		p <- p + geom_point(aes(color = p.adjust, size = Count))
		p <- p + scale_colour_gradient(low = colorHigh, high = colorLow, limit = c(min(data$p.adjust), max(data$p.adjust)), guide = guide_colourbar(reverse = TRUE))
	} else{
		p <- p + geom_point(aes(color = pvalue, size = Count))		
		p <- p + scale_colour_gradient(low = colorHigh, high = colorLow, limit = c(min(data$pvalue), max(data$pvalue)), guide = guide_colourbar(reverse = TRUE))
	}

	p <- p + labs(x = "GeneRatio", y = "", title = title) + theme(plot.title = element_text(hjust = 0.5))
	p <- p + theme(axis.text.y  = element_text(color = "black"))

	if(plot){
		pdf(out, w, h)
			print(p)
		dev.off()
	}

	return(p)
}

barplot2 <- function(data,
					 out = NULL, 
					 w = 6, 
					 h = 6,
					 n = 20,
					 title = "", 
					 padjust = TRUE, 
					 addLine = FALSE, 
					 fill = "#0089CB"
					 ){

	if(is.null(out))
		stop("Please specific parameter 'out' to save pdf file.\n")

	if(n > nrow(data))
		n = nrow(data)
	data = data.frame(data)[1:n, ]

	if(padjust){
		data = data[order(data$p.adjust), ]
		data$Description = factor(data$Description, levels = rev(data$Description))
		p <- ggplot(data) + setTheme() + setText(50)
		p <- p + geom_bar(aes(x = Description, y = -log2(p.adjust)), stat = 'identity', width = 0.8, fill = fill, alpha = 1)
	} else {
		data = data[order(data$pvalue), ]
		data$Description = factor(data$Description, levels = rev(data$Description))
		p <- ggplot(data) + setTheme() + setText(50)
		p <- p + geom_bar(aes(x = Description, y = -log2(pvalue)), stat = 'identity', width = 0.8, fill = fill, alpha = 1)

	}

	p <- p + labs(x = "", title = title) + coord_flip() + theme(plot.title = element_text(hjust = 0))
	p <- p + geom_hline(aes(yintercept = -log2(0.05)), colour = "black", linetype = "dashed", size = 1.5)
	p <- p + theme(axis.text.y = element_text(hjust = 1)) + theme(axis.text.y  = element_text(color = "black"))

	if(addCount){
		p <- p + geom_line(aes(x = Description, y = Count, group = 1), size = 2, color = "#BC0909")
		p <- p + geom_point(aes(x = Description, y = Count), size = 5)
	}

	pdf(out, w, h)
		print(p)
	dev.off()
}

simplifyAnno <- function(anno){

    res = rep(NA, length(anno))
    features = c("Promoter", "Exon", "Intron", "Intergenic", "UTR", "Downstream")
    for(feature in features) res[grep(feature, anno)] = feature
    res = factor(res, levels = features)
    return(res)
}

TCGAtranslateID <- function(file_ids, legacy = FALSE){

	suppressPackageStartupMessages(library("GenomicDataCommons"))
    info = files(legacy = legacy) %>%
           filter( ~ file_id %in% file_ids) %>%
           select('cases.samples.submitter_id') %>%
    id_list = lapply(info$cases, function(x) x[[1]][[1]][[1]])
    barcodes_per_file = sapply(id_list,length)
	res = data.frame(file_id = rep(ids(info),barcodes_per_file), 
					 submitter_id = unlist(id_list))
    return(res)
}

summaryHomer <- function(outFolder){

	homerFolder = paste0(outFolder, "/homerResults")
	xFiles = list.files(homerFolder, ".motif$")
	xFiles = xFiles[-grep("similar", xFiles)]
	xFiles = xFiles[-grep("RV", xFiles)]
	xFiles = xFiles[order(as.numeric(gsub("\\.", "", gsub("motif", "", xFiles))))]
	texts  = sapply(paste0(homerFolder, "/", xFiles), readLines)
	chunks = sapply(texts, function(x) strsplit(x[1], "[\t]"))

	motif = sapply(chunks, function(x) subString(x[1], 2, ">"))
	match = sapply(chunks, function(x) subString(subString(x[2], 2, "BestGuess:"),  1, "/"))
	score = sapply(chunks, function(x) rev(strsplit(x[2], "[()]")[[1]])[1])
	count = sapply(chunks, function(x) subString(x[6], 3, "[T:()]"))
	ratio = sapply(chunks, function(x) subString(x[6], 2, "[()]"))
	p_value = sapply(chunks, function(x) subString(x[6], 2, "P:"))

	xresT = data.frame(motif, 
					   match, 
					   score = as.numeric(score), 
					   count = as.numeric(count),
					   ratio_perc = as.numeric(gsub("%", "", ratio)), 
					   p_value = as.numeric(p_value)
					   )
	rownames(xresT) = gsub(".motif", "", basename(rownames(xresT)))
	return(xresT)
}

summaryHomerKnown <- function(outFolder){

	knownFolder = paste0(outFolder, "/knownResults")
	xFiles = list.files(knownFolder, ".motif$")
	xFiles = xFiles[order(as.numeric(gsub("\\.motif", "", gsub("known", "", xFiles))))]
	texts  = sapply(paste0(knownFolder, "/", xFiles), readLines)
	chunks = sapply(texts, function(x) strsplit(x[1], "[\t]"))

	motif = sapply(chunks, function(x) subString(x[1], 2, ">"))
	TF    = sapply(chunks, function(x) subString(x[2], 1, "/"))
	count = sapply(chunks, function(x) subString(x[6], 3, "[T:()]"))
	ratio = sapply(chunks, function(x) subString(x[6], 2, "[()]"))
	p_value = sapply(chunks, function(x) subString(x[6], 2, "P:"))

	xresT = data.frame(motif, 
					   TF, 
					   count = as.numeric(count),
					   ratio_perc = as.numeric(gsub("%", "", ratio)), 
					   p_value = as.numeric(p_value)
					   )
	rownames(xresT) = gsub("\\.motif", "", basename(rownames(xresT)))
	return(xresT)
}

ArrayIntervalMethylation <- function(MM, IntervalGR, cpgGR){

	# check seqlevels
	if(length(intersect(seqlevels(IntervalGR), seqlevels(cpgGR))) == 0)
		stop("No shared seqlevels found.\n")

	# only keep CpGs that locate in intervals
	Idx1 = countOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE) > 0
	Idx2 = names(cpgGR) %in% rownames(MM)
	Idx  = Idx1 & Idx2

	if(sum(Idx) == 0)
		stop("No CpGs found in pre-defined regions.\n")
	
	cpgGR = cpgGR[Idx, ]

	# keep shared CpGs
	cNames = intersect(rownames(MM), names(cpgGR))
	cpgGR  = cpgGR[cNames]
	MM    = MM[cNames, ]
	
	# Sample-by-sample processing to control memory footprint
	mergedM = matrix(NA, length(IntervalGR), ncol(MM))
	x = findOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE)
	aT  = aggregate(MM, by = list(factor(subjectHits(x))), FUN="mean0")
	aID = as.character(aT[,1])
	aMM = as.matrix(aT[,-1])
	mergedM[as.integer(aID), ] = aMM
	dimnames(mergedM) = list(as.character(IntervalGR), colnames(MM))
	
	## return
	return(mergedM)
}

runDME <- function(Matrix, s1, s2,
				   N_cutoff   = 3, 
				   FDRcutoff  = 0.05, 
				   TESTmethod = "wilcox",
				   diff       = 0.1,
				   padjust    = TRUE
				  ){

	# difference analysis
	betaI = betaII = delta = pvalue = padj = size = rname = rep(NA, nrow(Matrix))
	for(i in 1:nrow(Matrix)){

		x = as.numeric(na.omit(Matrix[i, s1]))
		y = as.numeric(na.omit(Matrix[i, s2]))
		if(length(x) < N_cutoff | length(y) < N_cutoff)
			next

		# if((length(unique(x)) == 1 & length(unique(y)) == 1))
		# 	if(unique(x) == unique(y))
		# 		next

		betaI[i]  = mean(x)
		betaII[i] = mean(y)
		delta = signif(betaI - betaII, 3)
		if(TESTmethod == "wilcox"){
			pvalue[i]   = suppressWarnings(wilcox.test(x, y)$p.value)
		} else if(TESTmethod == "t"){
			pvalue[i]   = suppressWarnings(t.test(x, y)$p.value)
		} else{
			stop("At present, only 'wilcox' and 't' test methods are available.")
		}
		size[i] = paste0(length(x), "/", length(y))
		rname[i] = rownames(Matrix)[i]
	}
	padj = p.adjust(pvalue, method = "fdr", n = length(pvalue))
	DEGtable = data.frame(betaI, betaII, delta, pvalue, padj, size)[!is.na(betaI), ]
	rownames(DEGtable) = na.omit(rname)
	DEGtable = na.omit(DEGtable)


	DGE = rep("NC", nrow(DEGtable))
	DGE[DEGtable$padj < FDRcutoff & DEGtable$delta > diff] = "UP"
	DGE[DEGtable$padj < FDRcutoff & DEGtable$delta < -diff] = "DN"
	DEGtable$DGE = DGE 
	return(DEGtable)
}

runDGE <- function(Matrix, s1, s2,
				   N_cutoff   = 3, 
				   FDRcutoff  = 0.05, 
				   TESTmethod = "wilcox",
				   DIFFmethod = "log2FC", 
				   PICKmethod = "cutoff",
				   TOPnumber  = 200,
				   DIFFcutoff = 2,
				   padjust    = TRUE
				  ){

	# check parameters
	if(DIFFmethod == "delta" & PICKmethod == "cutoff" & DIFFcutoff >= 1)
		stop("Please specify a value (range 0 to 1) for 'DIFFcutoff' when use 'delta' as method, 0.3 for example.")

	# difference analysis
	medianI = medianII = log2FC = delta = pvalue = padj = size = rep(NA, nrow(Matrix))
	for(i in 1:nrow(Matrix)){

		x = as.numeric(na.omit(Matrix[i, s1]))
		y = as.numeric(na.omit(Matrix[i, s2]))
		if(length(x) < N_cutoff | length(y) < N_cutoff)
			next

		if((length(unique(x)) == 1 & length(unique(y)) == 1))
			if(unique(x) == unique(y))
				next

		# medianI[i]  = median(x)
		# medianII[i] = median(y)
		# print(i)
		medianI[i]  = mean(x)
		medianII[i] = mean(y)
		if(TESTmethod == "wilcox"){
			pvalue[i]   = suppressWarnings(wilcox.test(x, y)$p.value)
		} else if(TESTmethod == "t"){
			pvalue[i]   = suppressWarnings(t.test(x, y)$p.value)
		} else{
			stop("At present, only 'wilcox' and 't' test methods are available.")
		}
		size[i]     = paste0(length(x), "/", length(y))
	}

	DEGtable = na.omit(data.frame(medianI, medianII, pvalue, size))

	# p.adjust
	pvalue2 = as.numeric(na.omit(pvalue))
	if(padjust){
		padj = p.adjust(pvalue2, method = "fdr", n = length(pvalue2))
	} else{
		padj = pvalue2
	}
	log10FDR = -log10(padj)

	# DIFFmethod
	if(DIFFmethod == "log2FC"){
		log2FC[!is.na(medianI)] = log2(medianI / medianII)	
		DEGtable = data.frame(DEGtable[, c(1, 2)], log2FC, pvalue, padj, log10FDR, size)
		# DEGtable = data.frame(medianI, medianII, log2FC, pvalue, padj, log10FDR, size)
	} else if(DIFFmethod == "delta"){
		delta = as.numeric(na.omit(medianI - medianII))
		DEGtable = data.frame(DEGtable[, c(1, 2)], delta, pvalue = na.omit(pvalue), padj, log10FDR, size = na.omit(size))
		# DEGtable = data.frame(medianI, medianII, delta, pvalue, padj, log10FDR, size)
	}

	# mark DGE
	DGE <- rep("NC", nrow(DEGtable))
	if(PICKmethod == "cutoff"){
		if(DIFFmethod == "log2FC"){
			DGE[((DEGtable$padj) < FDRcutoff) & (DEGtable$log2FC > DIFFcutoff)]  = "UP"
			DGE[((DEGtable$padj) < FDRcutoff) & (DEGtable$log2FC < -DIFFcutoff)] = "DN"
		} else if(DIFFmethod == "delta"){
			DGE[((DEGtable$padj) < FDRcutoff) & (DEGtable$delta > DIFFcutoff)]  = "UP"
			DGE[((DEGtable$padj) < FDRcutoff) & (DEGtable$delta < -DIFFcutoff)] = "DN"
		}
	} else if(PICKmethod == "top"){
		idx_padj = DEGtable$padj < FDRcutoff
		if(!sum(idx_padj)){
			message("\tNo significant feature be finded.")
			DEGtable$DGE = DGE
			return(DEGtable)
		} else if(2 * TOPnumber > sum(idx_padj)){
			TOPnumber = round(sum(idx_padj) / 2)
		}
		
		values = DEGtable$delta[idx_padj]
		idx_delta_UP = (values > 0) & (values > quantile(values, 1 - TOPnumber/length(values)))
		idx_delta_DN = (values < 0) & values < quantile(values, TOPnumber/length(values))

		DGE[which(idx_padj)[which(idx_delta_UP)]] = "UP"
		DGE[which(idx_padj)[which(idx_delta_DN)]] = "DN"
	}
	DEGtable$DGE = DGE

	rownames(DEGtable) = rownames(Matrix)
	res = na.omit(DEGtable)
	res[, 1:6] = apply(res[, 1:6], 2, function(x) signif(x, 4)) 

	return(res)
}

# DEGtable = runDGE(subBetaM, tumor, normal)
plotVolcano <- function(DEGtable, 
						DIFFcutoff = 2, 
						FDRcutoff = 0.05, 
						title = "DGE_volcano", 
						xtitle = "log2(FoldChange)", 
						ytitle = "-log10FDR",
						xlim = 5, 
						DIFFmethod = "log2", 
						textPos = 8.5){

	library("ggplot2")
	if(DIFFmethod == "log2"){
		p <- ggplot(DEGtable, aes(x = log2FC, y = log10FDR, color = DGE)) + xlim(-xlim, xlim)
	} else if(DIFFmethod == "delta"){
		p <- ggplot(DEGtable, aes(x = delta, y = log10FDR, color = DGE)) + xlim(-xlim, xlim)
	}
	p <- p + theme_bw() + labs(title = title, x = xtitle, y = ytitle)
	p <- p + geom_point(size = 0.1) + theme(plot.title = element_text(hjust = 0.5))
	p <- p + theme(panel.grid.major = element_blank(), panel.grid.minor = element_blank())
	p <- p + scale_color_manual(values = c("UP" = "red", "DN" = "dodgerblue4", "NC" = "grey"))
	p <- p + theme(legend.position = "none")

	if(DIFFmethod == "log2"){
		p <- p + geom_vline(xintercept = c(log2(DIFFcutoff), -log2(DIFFcutoff)), lty = 4, col = "black", lwd = 0.6)
	} else if(DIFFmethod == "delta"){
		p <- p + geom_vline(xintercept = c(-DIFFcutoff, DIFFcutoff), lty = 4, col = "black", lwd = 0.6)
	}
	p <- p + geom_hline(yintercept = -log10(FDRcutoff),lty = 4,col= "black", lwd= 0.6)

	N_DN = sum(DEGtable$DGE == "DN")
	p <- p + annotate("text", x = -2, y = textPos, label = paste("DN:", N_DN), size = 4, hjust = 1, color = "dodgerblue4",parse = TRUE)

	N_UP = sum(DEGtable$DGE == "UP")
	p <- p + annotate("text", x = 2, y = textPos, label = paste("UP:", N_UP), size = 4, hjust = 0, color = "red",parse = TRUE)

	return(p)
}

CGItoGene <- function(CGIs, DISTcutoff = 0){
	# check
	if(!grepl("\\chr", CGIs[1])[1]){
	        stop("CGIs format: chr1:135124-135563:+")
    }

	load("/sibcb2/bioinformatics2/wangjiahao/Data/RData/CGIanno.RData")
	Genes = as.character(CGIanno$Gene[CGIanno$CGI %in% CGIs])
	idx = Genes != "None"
	GenesNoneRemoved = Genes[idx]
	# if(length(which(!idx)))
		# cat("Warning:", length(which(!idx)), "CGIs no corresponding genes, removed.\n")
	resGene = unique(GenesNoneRemoved)
	if(DISTcutoff){
		subAnno = CGIanno[CGIanno$CGI %in% CGIs,]
		subAnnoNoNone = subAnno[Genes != "None",]
		idx = as.character(subAnnoNoNone$Dist) <= DISTcutoff
		Genes_too_far_removed = as.character(subAnnoNoNone$Gene[idx])
		# if(length(which(!idx)))
			# cat("Warning:", length(which(!idx)), "mapped genes too far away, removed.\n")
		resGene = unique(Genes_too_far_removed)
	}
	# cat("Result:", length(resGene), "genes acquired.\n")
	return(resGene)
}

PromoToGene <- function(Colnames){
	gene = sapply(strsplit(Colnames, ";"), function(x) x[[2]])
	return(unlist(gene))
}

convertId <- function(genes = NULL, from, to, OrgDb = "org.Hs.eg.db"){

	if(is.null(genes)){
		cat("\nAvailable gene types:\n")
		cat("\n\tACCNUM, ALIAS, ENSEMBL, ENSEMBLPROT, ENSEMBLTRANS, ENTREZID, ENZYME\n")
		cat("\n\tEVIDENCE, EVIDENCEALL, GENENAME, GO, GOALL, IPI, MAP, OMIM, ONTOLOGY\n")
		cat("\n\tONTOLOGYALL, PATH, PFAM, PMID, PROSITE, REFSEQ, SYMBOL, UCSCKG, UNIGENE, UNIPROT\n\n")
	}else{
		suppressPackageStartupMessages(library("clusterProfiler"))
		res = suppressMessages(suppressWarnings(
			  bitr(genes, fromType = from, toType = to, OrgDb = OrgDb)))
		if(length(to) == 1) res = unique(res[, 2])
		return(res)
	}
}


runEGO3 <- function(ENSEMBL){

	ego_BP = runEGO(ENSEMBL, ont = "BP")
	ego_BP2 = simplifyEGO(ego_BP, 15)[1:10]

	ego_CC = runEGO(ENSEMBL, ont = "CC")
	ego_CC2 = simplifyEGO(ego_CC, 15)[1:10]

	ego_MF = runEGO(ENSEMBL, ont = "MF")
	ego_MF2 = simplifyEGO(ego_MF, 15)[1:10]

	egoT2 = rbind(data.frame(ONTOLOGY = "BP", ego_BP2), data.frame(ONTOLOGY = "CC", ego_CC2), data.frame(ONTOLOGY = "MF", ego_MF2))
	data = egoT2[order(egoT2$Count, decreasing = TRUE), ]
	data$Description = factor(data$Description, levels = rev(data$Description))
	data$GeneRatioValue = as.numeric(subString(data$GeneRatio, 1, "/")) / as.numeric(subString(data$GeneRatio, 2, "/"))
	return(data)
}

dotplotGO <- function(data){

	data = data[order(data$p.adjust), ]
	data$Description = factor(data$Description, levels = rev(data$Description))
	data = data[order(data$GeneRatioValue, decreasing = TRUE), ]
	p <- ggplot(data, aes(x = GeneRatioValue, y = Description)) + theme_bw() + setText(25)
	p <- p + geom_point(aes(color = p.adjust, size = Count))
	p <- p + scale_colour_gradient(low = "#045A8D", high = "#F1EEF6", limit = c(min(data$p.adjust), max(data$p.adjust)), guide = guide_colourbar(reverse = TRUE))
	p <- p + labs(x = "GeneRatio", y = "", title = "")
	p <- p + theme(axis.text.y  = element_text(color = "black")) + theme(axis.text.y = element_text(hjust = 1))
	p <- p + facet_grid(ONTOLOGY ~ ., scale="free")
	p <- p + theme(strip.text.y = element_text(size = 15, angle = 0))
	p <- p + theme(strip.background.y = element_rect(fill = "white"))
	p <- p + theme(legend.key.size = unit(40, "pt"))
	p <- p + theme(legend.title=element_text(size=unit(25, "pt")))
	p <- p + theme(legend.text=element_text(size=unit(20, "pt")))
	return(p)
}

dotplot3 <- function(egoT, top = 20, padjust = TRUE, title = "GO enrichment"){

	library("gridExtra")

	data = egoT
	onts = unique(data$ONTOLOGY)

	plots = list()
	for(i in 1:length(onts)){

		ont = onts[i]
		xdata = data[data$ONTOLOGY == ont, ]
		xdata = xdata[order(xdata$Count, decreasing = TRUE), ]
		xdata$Description = factor(xdata$Description, levels = rev(xdata$Description))
		xdata$GeneRatioValue = as.numeric(subString(xdata$GeneRatio, 1, "/")) / as.numeric(subString(xdata$GeneRatio, 2, "/"))

		p <- ggplot(xdata, aes(x = GeneRatioValue, y = Description)) + theme_bw()
		if(padjust){
			p <- p + geom_point(aes(color = p.adjust, size = Count)) + setText(16)
			p <- p + scale_colour_gradient(low = "red", high = "blue", limit = c(min(data$p.adjust), max(data$p.adjust)), guide = guide_colourbar(reverse = TRUE))
		} else{
			p <- p + geom_point(aes(color = pvalue, size = Count))		
			p <- p + scale_colour_gradient(low = "red", high = "blue", limit = c(min(data$pvalue), max(data$pvalue)), guide = guide_colourbar(reverse = TRUE))
		}

		p <- p + labs(x = "GeneRatio", y = "", title = paste("GO:", ont))
		p <- p + theme(axis.text.y  = element_text(color = "black"))
		p <- p + theme(axis.text.y = element_text(hjust = 1))
		p <- p + theme(plot.title = element_text(hjust = 0.5))
		plots[[i]] = p
	}

	p_res <- grid.arrange(grobs = plots, ncol = 3) 
	return(p_res)

}

tosymbol <- function(geneid){symbol <- bitr(unlist(strsplit(geneid,split='/')),fromType='ENTREZID',toType='SYMBOL',OrgDb="org.Hs.eg.db")[,2];paste(symbol,collapse='/')}

trans <- function(df){generow <- df[,'geneID',drop=F]
                      geneid <- apply(generow,1,tosymbol)
                      outdf <- data.frame(df[,1:7],geneID=geneid,Count=df[,9])
                      return(outdf)
}


simplifyEGO <- function(ego, N = 20){

	if(ego@ontology == "GOALL"){



	} else{
		if(nrow(ego) <= N){
			check = FALSE
		} else{
			check = TRUE
		}
		cutoff = 0.7
		while(check){
			ego = simplify(ego, cutoff = cutoff)
			check = !nrow(ego) <= N
			cutoff = cutoff - 0.05
		}	
	}

	cat(nrow(ego), "terms Remain.\n")
	return(ego)
}

plotEGO <- function(ego, title = ""){
	p <- dotplot(ego, showCategory = nrow(ego)) + labs(title = title)
	p <- p + theme(plot.title = element_text(size = 20, face = "bold"))
	return(p)
}

conna.test <- function(value, group, cutoff = 3){

	Ns = length(unique(group))
	idx = 1
	for(i in 1:(Ns-1)){

		if(class(group) == "factor"){
			group1 = levels(group)[i]
		}else{
			group1 = unique(group)[i]
		}

		data1 = na.omit(value[group == group1])
		for(j in (i+1):Ns){

			if(class(group) == "factor"){
				group2 = levels(group)[j]
			}else{
				group2 = unique(group)[j]
			}

			data2  = na.omit(value[group == group2])
			if(length(data1) < cutoff | length(data2) < cutoff)
				next
			pair   = paste0(group1, "-", group2)
			medianI  = mean(data1)
			medianII = mean(data2)
			# medianI  = median(data1)
			# medianII = median(data2)
			diff   = medianI - medianII
			pvalue = wilcox.test(data1, data2, exact = FALSE)$p.value
			xresT  = data.frame(pair, medianI, medianII, diff, pvalue)
			# rownames(xresT) = pair
			if(idx == 1){
				resT = xresT
			} else{
				resT = rbind(resT, xresT)
			}
			idx = idx + 1
		}
		# resT = data.frame(resT)
	}
	
	if(idx == 1)
		return()
	resT$padj = signif(p.adjust(resT$pvalue, method = "fdr"), 4)
	return(na.omit(resT))
}
gghelp <- function(){

	cat('plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5)\n')
	cat('plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5)\n')
	cat('plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5)\n')
	cat('plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5)\n')
	cat('plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5)\n')
}

myR = "/sibcb2/bioinformatics2/wangjiahao/software/Miniconda3/bin/R"


mean0 = function(z) mean(z, na.rm = TRUE)

ArrayIntervalMethylation <- function(MM, IntervalGR, cpgGR){

	# check seqlevels
	if(length(intersect(seqlevels(IntervalGR), seqlevels(cpgGR))) == 0)
		stop("No shared seqlevels found.\n")

	# only keep CpGs that locate in intervals
	Idx1 = countOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE) > 0
	Idx2 = names(cpgGR) %in% rownames(MM)
	Idx  = Idx1 & Idx2

	if(sum(Idx) == 0)
		stop("No CpGs found in pre-defined regions.\n")
	
	cpgGR = cpgGR[Idx, ]

	# keep shared CpGs
	cNames = intersect(rownames(MM), names(cpgGR))
	cpgGR  = cpgGR[cNames]
	MM    = MM[cNames, ]
	
	# Sample-by-sample processing to control memory footprint
	mergedM = matrix(NA, length(IntervalGR), ncol(MM))
	x = findOverlaps(cpgGR, IntervalGR, type = "any", ignore.strand = TRUE)
	aT  = aggregate(MM, by = list(factor(subjectHits(x))), FUN="mean0")
	aID = as.character(aT[,1])
	aMM = as.matrix(aT[,-1])
	mergedM[as.integer(aID), ] = aMM
	colnames(mergedM) = colnames(MM)
	
	## return
	return(mergedM)
}

writeJSON <- function(xList, mainTag, fileOut){

	names(xList) = paste0(mainTag, ".", names(xList))
	NM  = names(xList)
	OUT = file(fileOut, "w")
	cat("{\n", file = OUT, append = FALSE, sep = "")

	for(i in 1:length(xList)){

		value = xList[[i]]
		if(i < length(xList))
			cat("\t\"", NM[i], "\":\"", value, "\",\n", file=OUT, append = TRUE, sep = "")
		else
			cat("\t\"", NM[i], "\":\"", value, "\"\n", file=OUT, append = TRUE, sep = "")
	}
	cat("}\n", file = OUT, append = TRUE, sep = "")

	close(OUT)
}

LeadingEdgeAnalysis <- function(Pvalue, OneGeneSet, cutoff, NumberOfPermutation = 10000) {
# Pvalue is the p-value vector with gene ID as names
# OneGeneSet is a gene ID vector
# cutoff the predefined cutoff score, smaller, more informative
	
	# keep only the genes that have Pvalue
	GeneID <- names( Pvalue )
	gsGene <- intersect( GeneID, OneGeneSet)
	M <- length(gsGene)
	# if(M < 5)
	#	return(NULL)

	# observed number
	oNumber <- sum( Pvalue[ gsGene ] < cutoff )

	# random sampled gene set
	rNumbers <- rep(0, NumberOfPermutation)
	for(i in 1:NumberOfPermutation) {

		temp <- sample(Pvalue, M)
		rNumbers[i] <- sum(temp < cutoff)
	}
	rNumber <- sum( oNumber < rNumbers)
	
	# return value
	rList <- list( gSize = M, oNumber = oNumber, rNumber = rNumber)
	return(rList)
}

LeadingEdgeAnalysisList <- function(Pvalue, pwList, quantileCutoff = 0.05, Nthreads = 16, NumberOfPermutation = 10000) {
# This fuction is used to test whether a function is enriched in a the top of a list

# Pvalue is the p-value vector with gene ID as names
# pwList is a pathway list. The element in the list should the same as the names of Pvalue
# quantileCutoff is a quantile value, default 0.05, top 5% genes considered to be significant

	# check required packges
	check1 <- ! require(foreach)
	check2 <- ! require(doMC)
	if(check1 | check2)
		stop("foreach and doMC are required for multi-threading.\n")

	# define the cutoff
	cutoff <- quantile(Pvalue, quantileCutoff)

	# multi-threading
	registerDoMC(Nthreads)
	N <- length(pwList)
	pwNames <- names(pwList)

	rList <- foreach(i = 1:N) %dopar% {

		cat("Processing pathway",i,",",pwNames[i],"\n\n")
		xx <- LeadingEdgeAnalysis(Pvalue, pwList[[i]], cutoff, NumberOfPermutation)
		xx
	}
	
	# processing the results 
	names(rList) <- pwNames
	# rList <- rList[! unlist( lapply(rList, function(x) is.null(x)) ) ]

	#return the results
	gsName <- names(rList)
	gsSize <- unlist( lapply(rList, function(x) x[1]))
	overlap<- unlist( lapply(rList, function(x) x[2]))
	p_value<- unlist( lapply(rList, function(x) x[3]))/NumberOfPermutation

	# return 
	outTable <- data.frame(gsName,gsSize,overlap,p_value) 
	return(outTable)
}

LeadingEdgeTopBot <- function(Pvalue, pwList, quantileCutoff = 0.05, minOverlap = 5, Nthreads = 16, NumberOfPermutation = 10000) {
# For a given gene list, test the pathways enriched in the top and botom
	
	# get the output for top enrichment
	mT <- LeadingEdgeAnalysisList(Pvalue, pwList, quantileCutoff, Nthreads, NumberOfPermutation)

	# get the output for botom enrichment
	mB <- LeadingEdgeAnalysisList(-Pvalue, pwList, quantileCutoff, Nthreads, NumberOfPermutation)

	# get each vector
	GeneSetName <- mT$gsName
	GeneSetSize <- mT$gsSize
	
	Overlap_top <- mT$overlap
	Pvalue_top  <- mT$p_value
	Pvalue_top_Masked <- Pvalue_top
	Pvalue_top_Masked[ Overlap_top < minOverlap] <- 1

	Overlap_bot <- mB$overlap
	Pvalue_bot  <- mB$p_value
	Pvalue_bot_Masked <- Pvalue_bot
	Pvalue_bot_Masked[ Overlap_bot < minOverlap] <- 1

	# merge Table
	mTable <- data.frame(GeneSetName,GeneSetSize,Overlap_top,Pvalue_top,Pvalue_top_Masked,Overlap_bot,Pvalue_bot,Pvalue_bot_Masked)
	return(mTable)
}

LeadingEdgeAnalysisDB <- function(Pvalue, pwListDB, quantileCutoff = 0.05, minOverlap = 5, Nthreads = 16, NumberOfPermutation = 10000) {
# For a given gene list, test the pathways enriched in the top and botom. pwListDB is a list of pwList

	# check whether pwListDB is list of list
	if( class(pwListDB) != "list" | class(pwListDB[[1]]) != "list")
		stop("The pwListDB has to be a list.\n")

	# test each pwList
	M <- length(pwListDB)
	cNames <- names(pwListDB)

	pList <- list()
	for(i in 1:M) {
		temp <- LeadingEdgeTopBot(Pvalue, pwListDB[[i]], quantileCutoff, minOverlap, Nthreads, NumberOfPermutation)
		className <- cNames[i]
		pList[[i]] <- data.frame(className, temp)
	}

	# combine them together
	mTable <- pList[[1]]
	if(M < 2)
		return(mTable)
	for(i in 2:M){
		mTable <- rbind(mTable, pList[[i]])
	}
	return(mTable)
}


OverlapAnalysis <- function(SG, SR, PG, NumberOfPermutation = 10000) {
# Get the significance of overlapping between SG and SR by permutation
	
	# check whether SG is subset of PG
	if( sum(! SG %in% PG) > 0)
		stop("SG should be included in PG.\n")

	# get the effecitve SG
	SR <- intersect(SR, PG)
	if(length(SR) < 5)
		return(NULL)

	# observed overlap
	oNumber <- sum(SG %in% SR)

	# random overlap
	rNumber <- rep(0, NumberOfPermutation)

	for(i in 1:NumberOfPermutation){
		temp <- sample(PG, length(SG) )
		rNumber[i] <- sum(temp %in% SR)
	}
	x <- sum(rNumber >= oNumber) + 1
	y <- max(NumberOfPermutation, x)
	Pvalue <- x/y

	# return
	sgSize <- length(SG)
	srSize <- length(SR)
	overlap<- oNumber
	p_value<- Pvalue

	rList <- list(sgSize = sgSize, srSize = srSize,overlap = overlap, p_value = p_value)
	return(rList)
}

OverlapAnalysisList <- function(SG, pwList, PG, Nthreads = 16, NumberOfPermutation = 1000) {

	# check required packges
	check1 <- ! require(foreach)
	check2 <- ! require(doMC)
	if(check1 | check2)
		stop("foreach and doMC are required for multi-threading.\n")
	registerDoMC(Nthreads)

	# foreach
	N <- length(pwList)
	gsNames <- names(pwList)

	rList <- foreach(i = 1:N) %dopar% {
		
		cat("Processing pathway",i,",",gsNames[i],"\n",sep = "")
		OverlapAnalysis(SG, pwList[[i]], PG, NumberOfPermutation)	
	}

	# combine
	rM <- matrix(NaN, N,4)
	for(i in 1:N) {

		temp <- rList[[i]]
		if( is.null(temp))
			next

		rM[i, 1] <- temp$sgSize
		rM[i, 2] <- temp$srSize
		rM[i, 3] <- temp$overlap
		rM[i, 4] <- temp$p_value
	}
	dimnames(rM) <- list(names(pwList), c("sgSize","srSize","overlap","p_value") )
	return(rM)
}

OverlapAnalysisListDB <- function(SG, pwListDB, PG, Nthreads = 16, NumberOfPermutation = 1000) {

	# process each pwList
	rList <- list()
	N <- length(pwListDB)

	for(i in 1:N) {
		rList[[i]] <- OverlapAnalysisList(SG, pwListDB[[i]], PG, Nthreads, NumberOfPermutation)
	}

	# merge
	dbNames   <- names(pwListDB)
	className <- rep(dbNames[1],  nrow(rList[[1]]))
	rM <- rList[[1]]
	outTable  <- data.frame(className, rM) 
	if(N == 1)
		return(outTable)

	#
	for(i in 2:N) {

		rM <- rbind(rM, rList[[i]])
		className <- c(className, rep(dbNames[i], nrow(rList[[i]])))
	}
	outTable <- data.frame(className, rM)
	return(outTable)
}

# serveral functions to performe hypergenomic enrichment on Signature genes
pHyperSR <- function(SG,SR,PG) {

	# keep only the overlap
	SG <- intersect(SG, PG)
	SR <- intersect(SR, PG)

	# pHyper
	m  <- length(SR)
	n  <- length(PG) - m
	k  <- length(SG)
	q  <- length( intersect(SG,SR) )

	pvalue <- phyper(q, m, n, k, lower.tail = FALSE, log.p = FALSE)
	
	#
	rList <- list( k = k, m = m, q = q, overlap = intersect(SG,SR), pvalue = pvalue)
	return(rList)
}

pHyperList <- function( SG, pwList, PG) {

	# define output
	N = length(pwList)
	rM <- matrix(NaN, N, 5)

	overlapVector <- rep("", N)

	for(i in 1:N) {

		SR <- pwList[[i]]
		rList <- pHyperSR(SG, SR, PG)

		rM[i, 1] <- rList$k
		rM[i, 2] <- rList$m
		rM[i, 3] <- rList$q
		rM[i, 4] <- rList$pvalue
		overlapVector[i] <- paste(rList$overlap,collapse = " ")
	}

	#q-value
	rM[ rM[,2] == 0 , 4] <- 1
	pvalue <- as.numeric(rM[, 4])
	
	qvalue <- pvalue*N/rank(pvalue)
	qvalue[ qvalue > 1] <- 1

	rM[,5] <- qvalue

	#
	rownames(rM) <- names(pwList)
	colnames(rM) <- c("sg","sr","n","pvalue","qvalue")
	
	return( data.frame(rM, overlapVector) )
}

pHyperListDB <- function( SG, pwListDB, PG, Annotation = FALSE) {

	# output
	M <- length(pwListDB)
	if(M < 2)
		stop("There are only one List in the pwListDB.")

	rM    <- pHyperList( SG, pwListDB[[1]], PG)
	pName <- names(pwListDB[[1]])
	cName <- rep(names(pwListDB)[1], nrow(rM) )
	rM    <- data.frame(pName, cName, rM)
	rownames(rM) <- NULL

	for(i in 2:M) {

		temp  <- pHyperList( SG, pwListDB[[i]], PG)
		pName <- names(pwListDB[[i]])
		cName <- rep(names(pwListDB)[i], nrow(temp) )
		temp  <- data.frame(pName, cName, temp)
		rownames(temp) <- NULL
		
		rM   <- rbind(rM, temp)
	}  

	#
	pvalue <- as.numeric(rM[,6])
	
	qvalue <- pvalue*nrow(rM)/rank(pvalue)
	qvalue[ qvalue > 1] <- 1

	rM[,7] <- qvalue

	# RETURN
	colnames(rM) <- c("PathwayName","DataBase","SignatureSize","PathwaySize","Overlap","Pvalue","Qvalue","OverlapGenes")
	return(rM)
}

getGeneByAbsEx <- function(geneName, exV) {
# For a given name vector and FC vector, remove the redanducy, keep the one with large abs FC
	
	if( length(geneName) != length(exV))
		stop("Genes and names must be of the same length.\n")
	if( class(geneName) != "character")
		stop("Gene names should be characters.\n")
	if( class(exV) != "numeric")
		stop("Expression must be numeric.\n")

	N <- length(geneName)
	Np<- 1:N

	geneU <- unique(geneName)
	M <- length(geneU)

	# define the output
	IndexP <- rep(0, M)

	for(i in 1:M) {

		Index1 <- geneName == geneU[i]
		if( sum(Index1) == 1)
			IndexP[i] <- Np[ Index1 ]
		else{
			subP <- Np[ Index1 ]
			subV <- exV[Index1 ]
			maxv <- max( abs(subV) )
			flag <- which(abs(subV) == maxv)[1]
			IndexP[i]<- subP[flag]
		}
	}

	#return the results
	rList <- list(geneU = geneU, IndexP = IndexP)
	return(rList)
}

SimpleRankTest <- function(Pvalue, gsList, size = 4) {
# The Pvalue is pvalue vector with Entrez ID as names
# gsList is the gene set list

	GeneID <- names(Pvalue)
	M <- length(gsList)
	zscore <- rep(NaN,M)
	
	N <- length(Pvalue)
	RankP <- rank(Pvalue,na.last = TRUE)

	pb<-txtProgressBar(min = 0, max = M, style = 3)
	
	for(i in 1:M) {

		Index <- GeneID %in% gsList[[i]] 
		n1 <- sum( Index )
		
		if(n1 < size)
			next
		n2 <- N - n1
		R1 <- sum(RankP[Index])
		K1 <- R1 - n1*(n1+1)/2
		Umean <- n1*n2/2
		Uvar  <- sqrt(n1*n2*(n1+n2+1)/12)
		zscore[i] <- (K1-Umean)/Uvar

		setTxtProgressBar(pb, i)
	}
	cat("\n\n")

	# return values
	names(zscore) <- names(gsList)
	return(zscore)
}

SimpleRankTestPermutation <- function(Pvalue, gsList, size = 4, NumberOfPermutation = 10000){

	# get all possible sizes
	sizeSet <- unique(sapply(gsList, length))
	sizeSet <- sizeSet[sizeSet > size]
	if(length(sizeSet) == 0)
		return(NULL)

	Nc <- length(sizeSet)
	rM <- matrix(NA, NumberOfPermutation, Nc)
	GID<- names(Pvalue)

	for(i in 1:Nc){

		randomList <- lapply(1:NumberOfPermutation, function(x) sample(GID, sizeSet[i]))
		rM[,i]     <- SimpleRankTest(Pvalue, randomList, size)	
	}
	colnames(rM) <- as.character(sizeSet)
	rMean<- apply(rM, 2, mean)
	rSD  <- apply(rM, 2, sd)

	# get observed z-score
	oZ   = SimpleRankTest(Pvalue, gsList, size)

	Ngs  = length(oZ)
	perZ = rep(NA, Ngs)
	perP = rep(NA, Ngs)

	for(i in 1:Ngs){

		gs      =  gsList[[i]]
		Tag     = as.character(length(gs))
		randomZ = rM[,Tag]

		perZ[i] = (oZ[i] - rMean[Tag])/rSD[Tag]
		pL      = sum(oZ[i] < randomZ)/NumberOfPermutation
		pR      = sum(oZ[i] > randomZ)/NumberOfPermutation
		perP[i] = min(pL, pR)
	} 

	# return
	return(list(oZ = oZ, perZ = perZ, perP = perP))
}

pwDB2List <- function(pwListDB){

	className <- names(pwListDB)
	if( length(className) == 1){
		
		rList 		 <- pwListDB[[1]]
		names(rList) <- paste(className[1],names(rList)) 
		return(rList)

	} else{

		rList <- list()
		N 	  <- length(className)
		for(i in 1:N){

			temp        <- pwListDB[[i]]
			names(temp) <- paste(className[i],names(temp))
			rList 	    <- append(rList,temp)
		}
		return(rList)
	}
}

pwListFilter <- function(pwList, PG, minN = 20, maxN = 2000){

	pwL <- list()
	for(i in 1:length(pwList)){
		temp <- intersect(pwList[[i]], PG)
		pwL[[i]] <- temp
	}
	names(pwL) <- names(pwList)
	
	# Filter
	Index <- sapply(pwL, function(x) length(x) >= minN) & sapply(pwL, function(x) length(x) <= maxN)
	return(pwL[Index])
}

GetRandomCorrelationMatrix <- function(Ex, PhenotypeVector, permutationN) {

	Ex <- as.matrix(Ex)
	PhenotypeVector <- as.numeric(PhenotypeVector)

	if( ncol(Ex) != length(PhenotypeVector))
		stop("Ex and PhenotypeVector doesn't match.\n")

	if( sum(is.na(PhenotypeVector)) > 0)
		stop("PhenotypeVector contains NaN value.\n")

	N <- nrow(Ex)
	M <- ncol(Ex)

	rM <- matrix(0, N,permutationN)
	observedCor <- rep(0, N)

	# permutation
	for(i in 1:permutationN) {

		cat("Permuation ",i,"\n")
		tempPhenotypeVector <- sample(PhenotypeVector, M)
		for(j in 1:N) {
			rM[j,i] <- cor(Ex[j,], tempPhenotypeVector)
		}	
	}
	rownames(rM) <- rownames(Ex)
	return(rM)
}

SimpleRankTestDB <- function(PvalueVector, pwListDB){

	M <- length(pwListDB)
	if(M <= 1)
		stop("There are less than 2 lists in pwListDB, use SimpleRankTest instead.\n")

	rList  <- list()
	GeneID <- names(PvalueVector)
	for(i in 1:M){

		pwList  <- pwListDB[[i]]
		zVector <- SimpleRankTest(PvalueVector, pwList)
		nVector <- sapply(pwList, function(x) length(intersect(x, GeneID)))
		pwNames <- names(pwList)
		cNames  <- rep(names(pwListDB)[i], length(pwList))
		rM      <- data.frame(pwNames, cNames, nVector, zVector)
		rownames(rM) <- NULL
		rList[[i]] <- rM
	}

	# MERGE
	rM <- rList[[1]]
	for(i in 2:M)
		rM <- rbind(rM, rList[[i]])

	return(rM)
}

GSEA.EnrichmentScore <- function(gene.list, gene.set, weighted.score.type = 1, correl.vector = NULL) {  
#
# Computes the weighted GSEA score of gene.set in gene.list. 
# The weighted score type is the exponent of the correlation 
# weight: 0 (unweighted = Kolmogorov-Smirnov), 1 (weighted), and 2 (over-weighted). When the score type is 1 or 2 it is 
# necessary to input the correlation vector with the values in the same order as in the gene list.
#
# Inputs:
#   gene.list: The ordered gene list (e.g. integers indicating the original position in the input dataset)  
#   gene.set: A gene set (e.g. integers indicating the location of those genes in the input dataset) 
#   weighted.score.type: Type of score: weight: 0 (unweighted = Kolmogorov-Smirnov), 1 (weighted), and 2 (over-weighted)  
#  correl.vector: A vector with the coorelations (e.g. signal to noise scores) corresponding to the genes in the gene list 
#
# Outputs:
#   ES: Enrichment score (real number between -1 and +1) 
#   arg.ES: Location in gene.list where the peak running enrichment occurs (peak of the "mountain") 
#   RES: Numerical vector containing the running enrichment score for all locations in the gene list 
#   tag.indicator: Binary vector indicating the location of the gene sets (1's) in the gene list 
#
# The Broad Institute
# SOFTWARE COPYRIGHT NOTICE AGREEMENT
# This software and its documentation are copyright 2003 by the
# Broad Institute/Massachusetts Institute of Technology.
# All rights are reserved.
#
# This software is supplied without any warranty or guaranteed support
# whatsoever. Neither the Broad Institute nor MIT can be responsible for
# its use, misuse, or functionality.

   tag.indicator <- sign(match(gene.list, gene.set, nomatch=0))    # notice that the sign is 0 (no tag) or 1 (tag) 
   no.tag.indicator <- 1 - tag.indicator 
   N <- length(gene.list) 
   Nh <- length(gene.set) 
   Nm <-  N - Nh 
   if (weighted.score.type == 0) {
      correl.vector <- rep(1, N)
   }
   alpha <- weighted.score.type
   correl.vector <- abs(correl.vector**alpha)
   sum.correl.tag    <- sum(correl.vector[tag.indicator == 1])
   norm.tag    <- 1.0/sum.correl.tag
   norm.no.tag <- 1.0/Nm
   RES <- cumsum(tag.indicator * correl.vector * norm.tag - no.tag.indicator * norm.no.tag)      
   max.ES <- max(RES)
   min.ES <- min(RES)
   if (max.ES > - min.ES) {
#      ES <- max.ES
      ES <- signif(max.ES, digits = 5)
      arg.ES <- which.max(RES)
   } else {
#      ES <- min.ES
      ES <- signif(min.ES, digits=5)
      arg.ES <- which.min(RES)
   }
   return(list(ES = ES, arg.ES = arg.ES, RES = RES, indicator = tag.indicator))    
}

GSEA.Permuation <- function(gene.list, gene.set,weighted.score.type = 1, correl.vector = NULL, permutationN = 1000) {

# remove genes not in gene.list
 gene.set <- intersect(gene.list, gene.set)
 M        <- length(gene.set)

 oES <- GSEA.EnrichmentScore(gene.list, gene.set, weighted.score.type = 1, correl.vector)$ES

# get random score
 rES <- rep(0, permutationN)
 for(i in 1:permutationN){
 	temp.set <- sample(gene.list, M)
 	rES[i]   <- GSEA.EnrichmentScore(gene.list, temp.set, weighted.score.type = 1, correl.vector)$ES
 }

 # return
 rList <- list(ES = oES, rES = rES)
 return(rList)
}

GSEA.Plot <- function(gseaReturnList, title = ""){
# gseaReturnList returned from GSEA.EnrichmentScore

	# get necessary score
	ES <- gseaReturnList$ES
	RES<- gseaReturnList$RES
	ind<- gseaReturnList$arg.ES
	POS<- gseaReturnList$indicator
	RK <- 1:length(RES)

	# ggplot2
	library("ggplot2")
	mTable <- data.frame(cbind(RK, RES, POS))
	p <- ggplot(mTable, aes(x = RK, y = RES) )
	p <- p + theme_bw() + labs(x = "Rank in Ordered Dataset", y = "Enrichment Score (ES)", title = title) 
	p <- p + geom_line(colour = "#2ca25f", size = 1.5)

	## indicator
	x <- RK[POS == 1]
	y <- rep(0, sum(POS))
	xend   <-  RK[POS == 1]
	yend   <-  rep(ES*0.25, sum(POS == 1))
	sTable <- data.frame(x, xend, y, yend)
	p <- p + geom_segment(data = sTable, aes(x = x, xend = xend, y = y, yend = yend), colour = "#3182bd", size = 0.5,alpha = 0.8)

	## add ES information
	x <- ind
	y <- 0
	xend <- ind
	yend <- ES
	eTable <- data.frame(x,xend,y,yend)
	p <- p + geom_segment(data = eTable, aes(x = x, xend = xend, y = 0, yend = yend), colour = "#dd1c77", size = 1)

	# return
	return(p)
}

SingleSampleGSEA_OLD <- function(fcMatrix, pwList, minN = 20, maxN = 20000, threadN = 8){

  # check required packges
  check1 <- ! require(foreach)
  check2 <- ! require(doMC)
  if(check1 | check2)
    stop("foreach and doMC are required for multi-threading.\n")
  registerDoMC(threadN)

  # keep only genes in the data set
  PG    <- rownames(fcMatrix)
  pwNew <- list()
  for(i in 1:length(pwList))
  	pwNew[[i]] <- intersect(PG, pwList[[i]])
  names(pwNew) <- names(pwList)

  Filter <- (sapply(pwNew, function(x) length(x)) > minN) & (sapply(pwNew, function(x) length(x)) < maxN)
  if(sum(Filter) == 0)
  	stop("No genes found in the matrix.\n")
  pwNew  <- pwNew[Filter]

  # Single sample GSEA
  N   <- length(pwNew)
  M   <- ncol(fcMatrix)
  ESM <- matrix(0, N, M)
  
  for(i in 1:M){

  	logFC <- fcMatrix[,i]
  	names(logFC) <- rownames(fcMatrix)
  	logFC        <- sort(logFC, decreasing = T)
  	gene.list    <- names(logFC)

  	xx <- foreach(j = 1:N) %dopar% {
  		gene.set <- pwNew[[j]]
  		GSEA.EnrichmentScore(gene.list, gene.set, weighted.score.type = 1, logFC)$ES
  	}
  	ESM[,i] <- unlist(xx)
  }
  dimnames(ESM) <- list(names(pwNew), colnames(fcMatrix))

  # return
  return(ESM)
}

SingleSampleGSEA <- function(fcMatrix, pwList){

  # Please make sure all genes in pwList are included in fcMatrix, and also in size of 20-2000
  pwNew  <- pwList

  # Single sample GSEA
  N   <- length(pwNew)
  M   <- ncol(fcMatrix)
  ESM <- matrix(0, N, M)
  
  for(i in 1:M){

  	logFC <- fcMatrix[,i]
  	names(logFC) <- rownames(fcMatrix)
  	logFC        <- sort(logFC, decreasing = T)
  	gene.list    <- names(logFC)

  	xx <- foreach(j = 1:N) %dopar% {
  		gene.set <- pwNew[[j]]
  		GSEA.EnrichmentScore(gene.list, gene.set, weighted.score.type = 1, logFC)$ES
  	}
  	ESM[,i] <- unlist(xx)
  }
  dimnames(ESM) <- list(names(pwNew), colnames(fcMatrix))

  # return
  return(ESM)
}

RandomSingleSampleGSEA <- function(fcMatrix, Ng, NN = 1000){

# Ng is the gene set size
	cat("running background distribution for size", Ng,"\n")

	PG <- rownames(fcMatrix)

	# generate random gene set
	randomList <- list()
	for(i in 1:NN){
		randomList[[i]] <- sample(PG, Ng)
	}
	names(randomList) <- paste("R", 1:NN,sep = "_")

	# random enrichment score
	esRM     <- SingleSampleGSEA(fcMatrix, randomList)
	BaseMean <- apply(esRM, 2, mean)
	BaseSD   <- apply(esRM, 2, sd)
	
	# return value
	rList <- list(BaseMean = BaseMean, BaseSD = BaseSD, Ng = Ng)	
	return(rList)
}

WorkflowSingleSampleGSEA <- function(fcMatrix, pwList, minN = 20, maxN = 2000, NN = 500, threadN = 32){

	# check required packges
  	check1 <- ! require(foreach)
  	check2 <- ! require(doMC)
  	if(check1 | check2)
    	stop("foreach and doMC are required for multi-threading.\n")
  	registerDoMC(threadN)

	# filter out by PG and size
	pwL    <- pwListFilter(pwList, rownames(fcMatrix), minN, maxN)

	# observed enrichment score
	esM    <- SingleSampleGSEA(fcMatrix, pwL)

	# check all possible size
	sizeA  <- sapply(pwL, function(x) length(x))
	sizeV  <- unique(sizeA)
	Ns     <- length(sizeV)
	rMeanM <- matrix(NA, Ns, ncol(fcMatrix))
	rSdM   <- matrix(NA, Ns, ncol(fcMatrix)) 

	for(i in 1:Ns){
		temp       <- RandomSingleSampleGSEA(fcMatrix, sizeV[i], NN)
		rMeanM[i,] <- temp$BaseMean
		rSdM[i,]   <- temp$BaseSD 
	}
	dimnames(rMeanM) <- list(as.character(sizeV), colnames(fcMatrix))
	dimnames(rSdM)   <- list(as.character(sizeV), colnames(fcMatrix))


	# get size-matched background
	rMeanM <- rMeanM[as.character(sizeA),]
	rSdM   <- rSdM[as.character(sizeA),  ]

	# Normalization
	resM   <- (esM - rMeanM)/rSdM

	# return
	rList <- list(esM = esM, resM = resM)
	return(rList)
}

ProjectToGeneSet <- function(
    data.array,
    # data.matrix containing gene expression data

    # exponential weight applied to ranking in calculation of enrichment score
    weight = 0,

    # gene set projecting expression data to
    gene.set) {

    gene.names <- row.names(data.array)
    n.rows <- dim(data.array)[1]
    n.cols <- dim(data.array)[2]

    ES.vector <- vector(length=n.cols)
    ranked.expression <- vector(length=n.rows, mode="numeric")

    # Compute ES score for signatures in each sample
    for (sample.index in 1:n.cols) {
        # gene.list is permutation (list of row indices) of the normalized expression data, where
        # permutation places expression data in decreasing order
        # Note that in ssGSEA we rank genes by their expression level rather than by a measure of correlation
        # between expression profile and phenotype.
        gene.list <- order(data.array[, sample.index], decreasing=T)

        # gene.set2 contains the indices of the matching genes.
        # Note that when input GCT file is ATARiS-generated, elements of
        # gene.names may not be unique; the following code insures each element
        # of gene.names that is present in the gene.set is referenced in gene.set2
        gene.set2 <- seq(1:length(gene.names))[!is.na(match(gene.names, gene.set))]

        # transform the normalized expression data for a single sample into ranked (in decreasing order)
        # expression values
        if (weight == 0) {
            # don't bother doing calcuation, just set to 1
            ranked.expression <- rep(1, n.rows)
        } else if (weight > 0) {
            # calculate z.score of normalized (e.g., ranked) expression values
            x <- data.array[gene.list, sample.index]
            ranked.expression <- (x - mean(x))/sd(x)
        }

        # tag.indicator flags, within the ranked list of genes, those that are in the gene set
        tag.indicator <- sign(match(gene.list, gene.set2, nomatch=0))    # notice that the sign is 0 (no tag) or 1 (tag)
        no.tag.indicator <- 1 - tag.indicator
        N <- length(gene.list)
        Nh <- length(gene.set2)
        Nm <-  N - Nh
        # ind are indices into ranked.expression, whose values are in decreasing order, corresonding to
        # genes that are in the gene set
        ind = which(tag.indicator==1)
        ranked.expression <- abs(ranked.expression[ind])^weight

        sum.ranked.expression = sum(ranked.expression)
        # "up" represents the peaks in the mountain plot; i.e., increments in the running-sum
        up = ranked.expression/sum.ranked.expression
        # "gaps" contains the lengths of the gaps between ranked pathway genes
        gaps = (c(ind-1, N) - c(0, ind))
        # "down" contain the valleys in the mountain plot; i.e., the decrements in the running-sum
        down = gaps/Nm
        # calculate the cumulative sums at each of the ranked pathway genes
        RES = cumsum(c(up,up[Nh])-down)
        valleys = RES[1:Nh]-up

        max.ES = max(RES)
        min.ES = min(valleys)

        if( max.ES > -min.ES ){
            arg.ES <- which.max(RES)
        } else{
            arg.ES <- which.min(RES)
        }
        # calculates the area under RES by adding up areas of individual
        # rectangles + triangles
        gaps = gaps+1
        RES = c(valleys,0) * (gaps) + 0.5*( c(0,RES[1:Nh]) - c(valleys,0) ) * (gaps)
        ES = sum(RES)

        ES.vector[sample.index] <- ES

    }
    return(list(ES.vector = ES.vector))

} # end of Project.to.GeneSet


ProjectToGeneSetList <- function(Ex, pwListDB, weight = 0.75, minN = 20, maxN = 2000, threadN = 8){

	# check required packges
  	check1 <- ! require(foreach)
  	check2 <- ! require(doMC)
  	if(check1 | check2)
    	stop("foreach and doMC are required for multi-threading.\n")
  	registerDoMC(threadN)

	# filter genes in the pathways
	PG     <- rownames(Ex)
	sgList <- list()
	classV <- c()
	M      <- length(pwListDB)

	for(i in 1:M){

		temp  <- pwListDB[[i]]
		sgList<- append(sgList, pwListDB[[i]])
		classV<- c(classV, rep(names(pwListDB)[i], length(temp)))
	}
	N <- length(sgList)
	for(i in 1:N)
		sgList[[i]] <- intersect(sgList[[i]], PG)

	size <- sapply(sgList, function(x) length(x))
	Flag <- (size > minN) & (size < maxN)

	sgList <- sgList[Flag]
	classV <- classV[Flag]

	# run
	N  <- length(sgList)
	rM <- matrix(NA, N, ncol(Ex))

	xx <- foreach(i = 1:N) %dopar% {

		SG <- sgList[[i]]
		ProjectToGeneSet(Ex, weight = weight, SG)$ES.vector
	}
	for(i in 1:N)
		rM[i,]   <- xx[[i]]
	colnames(rM) <- colnames(Ex)

	# save
	pathwayName <- names(sgList)
	pathwayClass<- classV
	out <- data.frame(pathwayName, pathwayClass, rM)
	return(out)
}

WorkflowSingleSampleShRNA <- function(FoldChangeVector, annotationTable, controlFlag = "NonTargetingControlGuideForMouse", NN = 10000){
# FoldChangeVector is a vector of fold change with sgRNA names, which should all be included in annotationTable
# Below is the format of annotationTable
#                        Sequences         Plate
# MGLibA_00001 TCCTGAATGTGTTACGAAGC 0610007P14Rik
# MGLibA_00002 GGTCGGGCTCCGGTACCTAG 0610007P14Rik

	# check rownames
	check = sum(!(rownames(FoldChangeVector) %in% rownames(annotationTable))) == 0
	if(!check)
		stop("Some sgRNA could not be found in annotationTable.\n")

	# remove missing values
	Index = !is.na(FoldChangeVector)
	FoldChangeVector = FoldChangeVector[Index]

	# Build pathway list
	probeID = names(FoldChangeVector)
	geneID  = as.character(annotationTable[probeID,2])
	pwList  = split(probeID, geneID)
	Index   = sapply(pwList, function(x) length(x)) > 3
	pwList  = pwList[Index]

	# get z-score for all genes
	oZscore = SimpleRankTest(FoldChangeVector, pwList)

	# buid random z-score
	controlProbes = probeID[grep(controlFlag, geneID)]
	setSize       = unique(sapply(pwList, function(x) length(x)))
	randomM       = matrix(NA, NN, max(setSize))
	
	for(i in 1:length(setSize)){

		# build random gene sets
		randomList = list()
		for(j in 1:NN)
			randomList[[j]] = sample(controlProbes, setSize[i])
		names(randomList)   = paste0("R_", 1:NN)

		randomM[,setSize[i]] = SimpleRankTest(FoldChangeVector, randomList)
	}

	# convert z-score to p-value
	N       = length(oZscore)
	oPvalue = rep(NA, N)
	oNz     = rep(NA, N)

	GSsize  = sapply(pwList, function(x) length(x))
	rMean   = apply(randomM, 2, mean)
	rSD     = apply(randomM, 2, sd)

	for(i in 1:N){
		Flag = GSsize[i]
		rv = randomM[, Flag]
		p1 = (sum(oZscore[i] > rv) + 1)/NN
		p2 = (sum(oZscore[i] < rv) + 1)/NN
		oPvalue[i] = min(p1, p2)*2

		oNz[i] = (oZscore[i] - rMean[Flag])/rSD[Flag]
	}

	# return
	mT = cbind(oZscore, oPvalue, oNz)
	return(mT)
}

RankSumZscore <- function(test_Vector, control_Vector){

 	names(test_Vector)    = paste0("Test_", 1:length(test_Vector))
 	names(control_Vector) = paste0("Control_", 1:length(control_Vector))
 	all_Vector <- c(test_Vector, control_Vector)
 	test       <- names(test_Vector)
 	zscore = SimpleRankTest(all_Vector, list(test = test))[1]
 	return(zscore)
}

# functions used in ggplot2

myDonkey <- function(){

cat("\n") 
cat( "           \\  ^__^     hee-haw~           \n")
cat( "            \\ (oo)\\________         \n")
cat( "              (__)\\        )\\/\\    \n")
cat( "                  ||-----w |          \n")
cat( "                  ||      ||          \n\n")
}

pitchPlot <- function(){

	if(FALSE){
		library("gridExtra")
		pdf(paste0("PDF/TCGA_mutate_driver_", treat, ".pdf"), 15, 9)
			grid.arrange(grobs = plots, ncol = 8) 
		dev.off()
	}
}


# --------->	text    <---------
setText <- function(title.size   = 10,
                    x.title.size = NA,
                    y.title.size = NA,
                    x.text.size  = NA,
                    y.text.size  = NA,
                    face         = "plain",
                    x.title.face = NA,
                    y.title.face = NA,
                    x.text.angle = 0,
                    y.text.angle = 0,
                    x.text.hjust = 0.5,
                    y.text.hjust = 0
){
	
    checkNA <- function(x, y, ratio = NA){
        
        if(is.na(x))
            if(is.na(ratio)){
                x = y
            } else{
                x = y * ratio
            }
        return(x)
    }
    
    x.title.size = checkNA(x.title.size, title.size, 0.8)
    y.title.size = checkNA(y.title.size, title.size, 0.8)
    x.text.size  = checkNA(x.text.size, title.size, 0.6)
    y.text.size  = checkNA(y.text.size, title.size, 0.6)
    x.title.face = checkNA(x.title.face, face)
    y.title.face = checkNA(y.title.face, face)
    
    if(x.text.angle)
        x.text.hjust = 1
    if(y.text.angle)
        y.text.hjust = 1
    
    p <- theme(plot.title   = element_text(color = "black", size = title.size, face = face, hjust = 0.5),
               axis.title.x = element_text(color = "black", size = x.title.size, face = x.title.face),
               axis.title.y = element_text(color = "black", size = y.title.size, face = y.title.face, vjust = 1.2),
               axis.text.x  = element_text(size = x.text.size, angle = x.text.angle, hjust = x.text.hjust),
               axis.text.y  = element_text(size = y.text.size, angle = y.text.angle, hjust = y.text.hjust),
               plot.margin  = margin(t = 5.5, r = 5.5, b = 5.5, l = 5.5, unit = "pt"))

    return(p)
}


# --------->	element omit    <---------
setBlank <- function(x.title  = 1,
                     x.text   = 1,
                     x.ticks  = 1,
                     y.title  = 1,
                     y.text   = 1,
                     y.ticks  = 1
                    ){
    
    p = theme()
    if(x.title)
        p <- p + theme(axis.title.x = element_blank())
    if(x.text)
        p <- p + theme(axis.text.x = element_blank())
    if(x.text)
        p <- p + theme(axis.ticks.x = element_blank())
    
    if(y.title)
        p <- p + theme(axis.title.y = element_blank())
    if(y.text)
        p <- p + theme(axis.text.y = element_blank())
    if(y.ticks)
        p <- p + theme(axis.ticks.y = element_blank())
    
    return(p)
}


# --------->	axis limit    <---------
# bug: x y can not set simultaneously, due to scale_x/y can not plus together when not add with theme() firstly
setLimit <- function(xlim     = 0, 
					 ylim     = 0,
					 x.expand = 0,
					 y.expand = 0
					 ){

	if(length(xlim) != 1){
		if(length(xlim) == 2){
			if(length(x.expand) == 1){
				p <- scale_x_continuous(limits = c(xlim[1], xlim[2]))
			} else{
				p <- scale_x_continuous(limits = c(xlim[1], xlim[2]), expand = x.expand)
			}
		} else{
			if(length(x.expand) == 1){
				p <- scale_x_continuous(limits = c(xlim[1], xlim[2]), breaks = seq(xlim[1], xlim[2], xlim[3]))
			} else{
				p <- scale_x_continuous(limits = c(xlim[1], xlim[2]), breaks = seq(xlim[1], xlim[2], xlim[3]), expand = x.expand)
			}
		}
		if(length(ylim) != 1)
			if(length(ylim) == 2){
				if(length(y.expand) == 1){
					p <- p + scale_y_continuous(limits = c(ylim[1], ylim[2]))
				} else{
					p <- p + scale_y_continuous(limits = c(ylim[1], ylim[2]), expand = y.expand)
				}

			} else{
				if(length(y.expand) == 1){
					p <- p + scale_y_continuous(limits = c(ylim[1], ylim[2]), breaks = seq(ylim[1], ylim[2], ylim[3]))
				} else{
					p <- p + scale_y_continuous(limits = c(ylim[1], ylim[2]), breaks = seq(ylim[1], ylim[2], ylim[3]), expand = y.expand)
				}

			}
	} else{
		if(length(ylim) == 2){
			if(length(y.expand) == 1){
				p <- scale_y_continuous(limits = c(ylim[1], ylim[2]))
			} else{
				p <- scale_y_continuous(limits = c(ylim[1], ylim[2]), expand = y.expand)
			}

		} else{
			if(length(y.expand) == 1){
				p <- scale_y_continuous(limits = c(ylim[1], ylim[2]), breaks = seq(ylim[1], ylim[2], ylim[3]))
			} else{
				p <- scale_y_continuous(limits = c(ylim[1], ylim[2]), breaks = seq(ylim[1], ylim[2], ylim[3]), expand = y.expand)
			}

		}
	}

    return(p)
}


# --------->	legend    <---------
setLegend <- function(direction   = "vertical", 
					  position    = "right", # "none",  
					  title       = NA, # 
					  order       = NA, # c()
					  labels      = NA,
					  text.size   = 10,
					  text.col    = "black",
					  text.face   = "plain",
					  text.angle  = 0,
					  text.hjust  = 0,
					  text.vjust  = 0,
					  title.size  = 10,
					  title.col   = "black",
					  title.face  = "plain",
					  title.angle = 0,
					  title.hjust = 0,
					  title.vjust = 0,
					  key.size    = 2,
					  key.color   = NA,
					  key.fill    = NA,
					  margin      = 10
		 			 ){

  	# basic
	p <- theme(legend.direction = direction, 
			   legend.position  = position,
		  	   legend.text      = element_text(size = text.size, color = text.col, face = text.face, angle = text.angle, hjust = text.hjust, vjust = text.vjust),
		  	   legend.key       = element_rect(fill = key.fill, color = key.color),
		  	   legend.key.size  = unit(key.size, "cm"))

	# magin
  	if(length(margin) == 1){
	  	p <- p + theme(legend.margin = margin(rep(margin, 4), "pt"))
  	} else{
	  	p <- p + theme(legend.margin = margin(margin[1], margin[2], margin[3], margin[4], "pt"))
  	}

  	# title
  	if(is.na(title)){
	  	p <- p + theme(legend.title = element_text(size = title.size, color = title.col, face = title.face, angle = title.angle, hjust = title.hjust, vjust = title.vjust))
	} else if(isFALSE(title)){
  		p <- p + theme(legend.title = element_blank())
  	} else if(is.character(title)){
	  	p <- p + guides(color = guide_legend(title = title))
  	}

	# order, labels and palette
  	if(is.na(palette)){
		if(!is.na(order) & is.na(labels)){
			p <- p + scale_color_discrete(breaks = order)
		} else if(is.na(order) & !is.na(labels)){
			p <- p + scale_color_discrete(labels = labels)
		} else if(!is.na(order) & !is.na(labels)){
			p <- p + scale_color_discrete(breaks = order, labels = labels)
		}
  	} else{
  		if(!is.na(order) & is.na(labels)){
			p <- p + scale_color_manual(values = palette, breaks = order)
		} else if(is.na(order) & !is.na(labels)){
			p <- p + scale_color_manual(values = palette, labels = labels)
		} else if(!is.na(order) & !is.na(labels)){
			p <- p + scale_color_manual(values = palette, breaks = order, labels = labels)
		}
  	}

	return(p)
}


# --------->	theme    <---------
setTheme <- function(theme.bw   = TRUE, 
					 grid.major = FALSE, 
					 grid.minor = FALSE
					 ){
	p <- theme()
	if(theme.bw)
		p <- p + theme_bw() 

	if(!grid.major)
		p <- p + theme(panel.grid.major = element_blank())

	if(!grid.minor)
		p <- p + theme(panel.grid.minor = element_blank())
	
	return(p)
}



# setFacet <- function(x,
# 					 y,
# 					 text.x.color,
# 					 text.x.size,
# 					 text.y.color,
# 					 text.y.size,
# 					 background.x
# 					 background.y
					 
# 	){



# 	facet_grid(x ~ y)
# }


plotEmpty <- function(){
	p <- ggplot() + geom_point(aes(1, 1), colour = "white") +   
		 theme(axis.ticks = element_blank(), 
        	   panel.background = element_blank(), 
        	   axis.line = element_blank(), 
        	   axis.text.x = element_blank(), 
        	   axis.text.y = element_blank(), 
        	   axis.title.x = element_blank(), 
        	   axis.title.y = element_blank())
	return(p)
}
options(stringsAsFactors = FALSE, future.rng.onMisuse = "ignore", future.globals.maxSize = 80 * 1024^3)
future::plan(strategy = "multicore", workers = 20)
startSC <- function(lib = FALSE){

	if(lib) loadp(Seurat, ggplot2, dbplyr, RColorBrewer)
	print(paste('Start', as.character(date()), sep = '    '))
	source("/sibcb2/bioinformatics2/wangjiahao/code/workflow/scRNA_seq.R")
}

readfile2seurat <- function(file_path) # by fox

{
    loadp(data.table, Seurat)
    data <- fread(file_path)
    rownames(data) <- data$V1
    data$V1 <- NULL
    data <- CreateSeuratObject(counts = data, min.cell = 0)
    return(data)
}

show_cols <- function(col, plot = FALSE){

	loadp(ggplot2)
	p <- ggplot(data.frame(col = factor(col, levels = rev(col))), aes(col))
	p <- p + geom_bar(aes(fill = rev(col)), width = 1)
	p <- p + scale_fill_manual(values = col)
	p <- p + scale_y_continuous(expand = c(0, 0))
	p <- p + theme(legend.position = "none") + labs(x = NULL, y = NULL)
	p <- p + theme(axis.text.x = element_blank(), panel.background = element_blank(), axis.ticks = element_blank())
	p <- p + theme(axis.text.y = element_text(face = "bold", size = 15))
	p <- p + coord_flip()
	if(plot){
		pdf("show_cols.pdf", 3, 5)
			p
		dev.off()
		cat("File save to 'show_cols.pdf.'\n")
	}
	return(p)
}

readCounts <- function(file){

	library(data.table)
	counts = fread(file, check.names = FALSE)
	row.names = as.character(data.frame(counts[, 1])[, 1])
	counts[, 1] = NULL
	counts = data.frame(counts, check.names = FALSE)
	rownames(counts) = row.names
	return(counts)
}

readMeta <- function(file){

	meta <- read.table(file, sep = "\t", header = TRUE, row.names = 1, check.name = TRUE)
	return(meta)
}

creatSeuratObj <- function(countFile, metaFile){

	counts = readCounts(countFile)
	meta = readMeta(metaFile)
	obnj = CreateSeuratObject(counts = counts, meta.data = meta)
}



writeJSON <- function(xList, mainTag, fileOut){

	library("rjson")
	names(xList) = paste0(mainTag, ".", names(xList))
	NM  = names(xList)
	OUT = file(fileOut, "w")
	cat("{\n", file = OUT, append = FALSE, sep = "")

	for(i in 1:length(xList)){

		value = xList[[i]]
		if(i < length(xList))
			cat("\t\"", NM[i], "\":\"", value, "\",\n", file=OUT, append = TRUE, sep = "")
		else
			cat("\t\"", NM[i], "\":\"", value, "\"\n", file=OUT, append = TRUE, sep = "")
	}
	cat("}\n", file = OUT, append = TRUE, sep = "")

	close(OUT)
}

writeSH <- function(cmd, bashOut, jobName, LogFolder, Ncpu, Memory){

	OUT = file(bashOut, "w")
	
	cat("#!/bin/bash\n", file=OUT, append=FALSE, sep = "")
	cat("#$ -N ", jobName, "\n", file=OUT, append=TRUE, sep = "")
	cat("#$ -l h_cpu=720:00:00\n", file=OUT, append=TRUE, sep = "")	
	cat("#$ -V\n#$ -m n\n#$ -cwd\n", file=OUT, append=TRUE, sep = "")
	cat("#$ -pe smp ", Ncpu, "\n", file=OUT, append=TRUE, sep = "")
	cat("#$ -l h_vmem=", Memory, "G\n\n", file=OUT, append=TRUE, sep = "")
	cat(cmd, "\n", file=OUT, append=TRUE, sep = "")
	close(OUT)

	outFile = paste0(LogFolder,"/", jobName, ".out")
	errFile = paste0(LogFolder,"/", jobName, ".err")

	submitCMD = paste("qsub", "-o", outFile, "-e", errFile, bashOut)
	return(submitCMD)
}

buildSEG <- function(cmd, jobName, LogFolder, Ncpu, Memory){

	outFile = paste0(LogFolder,"/", jobName, ".out")
	errFile = paste0(LogFolder,"/", jobName, ".err")

	# queues = c("g5.q")
	queues = c("g1.q", "g3.q", "g5.q")
	submitCMD = paste0("echo ", "\"", cmd, "\"")
	submitCMD = paste(submitCMD, "| qsub -N", jobName, "-q", sample(queues, 1))
	submitCMD = paste(submitCMD, "-l h_cpu=720:00:00 -V -m n -cwd -pe smp", Ncpu)
	submitCMD = paste0(submitCMD, " -l h_vmem=", Memory, "G")
	submitCMD = paste(submitCMD, "-o", outFile, "-e", errFile)

	return(submitCMD)
}

SimpleSGE <- function(cmd, jobName, LogFolder){

	if(!dir.exists(LogFolder))
		dir.create(LogFolder)
	outFile = paste0(LogFolder,"/", jobName, ".out")
	errFile = paste0(LogFolder,"/", jobName, ".err")

	# queues = c("g1.q", "g3.q", "g4.q", "g5.q")
	# queues = c("g1.q", "g3.q", "g5.q") # g4 got Eqw
	queues = c("g5.q") # only g5 is not charge
	submitCMD = paste0("echo ", "\"", cmd, "\"")
	submitCMD = paste(submitCMD, "| qsub -N", jobName)
	submitCMD = paste(submitCMD, "-V -cwd -q", sample(queues, 1))
	submitCMD = paste(submitCMD, "-o", outFile, "-e", errFile)

	return(submitCMD)
}
