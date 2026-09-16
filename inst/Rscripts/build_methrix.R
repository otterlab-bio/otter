library(optparse)
library(methrix)
library(dplyr)
library(BSgenome)
option_list = list(make_option(c("--filein"), type = "character", default = NULL, help = "CpG Island BED file, provided by default."),
                   make_option(c("--fileout"), type = "character", default = "human", help = "species1, provided by default."),
                   make_option(c("--cores"), type = "character", default = "10", help = "cores, provided by default."),
                   make_option(c("--Genome"), type = "character", default = "hg19", help = "hg19, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
filein <- args$filein
fileout <- args$fileout
cores <- args$cores %>%
  as.numeric()
genome <-  stringr::str_to_lower(args$Genome)
message(fileout)
fs::dir_create(fileout)
temp_dir <- paste0(fileout,"temp")
fs::dir_create(temp_dir)

if (genome %in% c("hg19","Hg19","HG19")){
  ref_genome = "BSgenome.Hsapiens.UCSC.hg19"
}else if (genome %in% c("hg38","Hg38","HG38")){
  ref_genome = "BSgenome.Hsapiens.UCSC.hg38"
}else if (genome %in% c("hg17","Hg17","HG17")){
  ref_genome = "BSgenome.Hsapiens.UCSC.hg17"
}else if (genome %in% c("GRCm38","GRCM38","grcm38")){
  ref_genome = "BSgenome.Mmusculus.UCSC.mm10"
}else if (genome %in% c("mm39","MM39","Mm39")){
  ref_genome = "BSgenome.Mmusculus.UCSC.mm39"
}else{
  ref_genome = NULL
}

if (is.null(ref_genome)){
  hg19_cpgs = NULL
}else{
  if (!requireNamespace(ref_genome, quietly = TRUE)) {
    pak::pak(ref_genome)
  }
  hg19_cpgs = methrix::extract_CPGs(ref_genome = ref_genome)
}


bismark_covfiles <- list.files(path = filein,"*.bismark.cov.gz$",full.names = T)
meth = methrix::read_bedgraphs(files = bismark_covfiles,
                               ref_cpgs = hg19_cpgs,
                               chr_idx = 2,
                               start_idx = 3,
                               M_idx = 5,
                               U_idx = 6,
                               stranded = TRUE,
                               collapse_strands = TRUE,
                               pipeline = "Bismark_cov",
                               zero_based=FALSE,
                               n_threads = cores,
                               h5 = TRUE,
                               h5_dir = fileout,
                               h5temp = temp_dir,
                               vect = F)
#fs::dir_delete(temp_dir)

methrix_obj_clean <- meth %>%
  methrix::remove_uncovered()
# CpG coverage

methrix_obj_coverage <- methrix::get_matrix(methrix_obj_clean,
                                            type="C",
                                            add_loci = F) %>%
  as.data.frame()

coverage_df <- NULL
for (i in 1:ncol(methrix_obj_coverage)){
  message(">> Processing Sample",i)
  tempdf <- table(methrix_obj_coverage[[colnames(methrix_obj_coverage)[i]]],useNA = "no")
  names(tempdf) <- paste0(names(tempdf),"X")
  subset_X <- c("1X","2X","3X","4X","5X","10X")
  subset_X <- subset_X[subset_X %in% names(tempdf) ]

  cover_list_tbl <- list()
  for (a in 1:length(subset_X)){
    if (a == 1){
      subset_X_idx <- c()
    }else{
      subset_X_idx <- paste0(1:a-1,"X")
    }
    subset_X_count <- setdiff(names(tempdf),subset_X_idx)

    tempcount <- tempdf[subset_X_count] %>%
      sum()

    cover_list_tbl[[a]] <- data.frame(
      names = subset_X[a],
      freq = tempcount
    )
  }

  tempdf_i <- data.table::rbindlist(cover_list_tbl) %>%
    as.data.frame()
  colnames(tempdf_i)[2] <- colnames(methrix_obj_coverage)[i] %>%
    stringr::str_remove(.,"_nsort")

  if (is.null(coverage_df)){
    coverage_df <- tempdf_i
  }else{
    coverage_df <- coverage_df %>%
      dplyr::left_join(tempdf_i,by = "names")
  }
}

coverage_df %>%
  tibble::column_to_rownames("names") %>%
  t() %>%
  as.data.frame() %>%
  tibble::rownames_to_column("sampleid") %>%
  dplyr::relocate(sampleid) %>%
  openxlsx::write.xlsx(.,
                       paste0(fileout,"/CpG_coverage.xlsx"))


meth %>%
  methrix::convert_HDF5_methrix() %>%
  methrix::methrix2bsseq() %>%
  saveRDS(.,
          file = paste0(fileout,"/bsseq.RDS"))
