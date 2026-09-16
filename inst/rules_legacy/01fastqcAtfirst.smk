rule fastqcAtfirst:
  message:"Running FastQC At first Glance ..."
  input:
    R1= lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R1.fastq.gz"),
    R2= lambda wildcards: os.path.join(config["output"]["raw_dir"], f"{wildcards.sample}_R2.fastq.gz")
  output:
    R1_fastqc_zip = os.path.join( config["directories"]["qc"]["before"], "{sample}" + "_R1_fastqc.zip"),
    R1_fastqc_html = os.path.join( config["directories"]["qc"]["before"], "{sample}" + "_R1_fastqc.html"),
    R2_fastqc_zip = os.path.join( config["directories"]["qc"]["before"], "{sample}" + "_R2_fastqc.zip"),
    R2_fastqc_html = os.path.join( config["directories"]["qc"]["before"], "{sample}" + "_R2_fastqc.html")
  params:
    dir=config["directories"]["qc"]["before"]
  threads: 6
  shell:
    """
    enva run fastqc -- fastqc -o {params.dir} -t {threads} --extract {input.R1} {input.R2}
    
    """
